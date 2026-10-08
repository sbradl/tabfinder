package dev.tabsync.tabfinder.data

import android.content.Context
import android.net.Uri
import android.os.Environment
import android.provider.DocumentsContract
import androidx.core.content.edit
import java.io.BufferedReader
import java.io.BufferedWriter
import java.io.File
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

// The JSON of `tabscan -serve` (cmd/tabscan/serve.go). The search and what the list shows of
// each song (internal/rows) live there, in Go, shared with the desktop app; these are its answers.

/** A tuning as shown: "Drop C" on a 6-string; [detail] is its notes unless the label is the notes. */
@Serializable data class Tuning(val strings: Int, val label: String, val detail: String = "")

@Serializable
data class Song(
  val path: String, // relative to the tab folder
  val title: String,
  val artist: String,
  val album: String,
  val tunings: List<Tuning> = emptyList(), // the default lets coerceInputValues read a null as empty
  val unreadable: Boolean,
  val openAs: String, // the file name to hand TuxGuitar a copy under
  val subtitle: String = "", // "artist · album", leaving out what's missing
  val tempo: String = "", // the opening tempo, shown large; empty for none
  val tempoDetail: String = "", // under it: "BPM", or the tempo changes that follow
  val parts: List<Part> = emptyList(), // drums, bass, rhythm, lead: those played
)

/** How hard what a [role] ("drums", "bass", "rhythm", "lead") plays in a song is: [level] 1 to 10. */
@Serializable data class Part(val role: String, val level: Int, val tags: List<String> = emptyList())

/** The order of the songs found. */
@Serializable
enum class Sort {
  @SerialName("") AZ, // by artist, then title
  @SerialName("easiest") EASIEST, // by the hardest part searched for (any if none), easiest first
  @SerialName("hardest") HARDEST,
}

/**
 * The filter fields as typed. [strings] is set by picking a tuning suggestion. [drums], [bass], [rhythm]
 * and [lead] are ranges of levels ("5-7"), each asking for that part; [sort] is no filter.
 */
@Serializable
data class Query(
  val name: String = "",
  val artist: String = "",
  val tuning: String = "",
  val bpm: String = "",
  val strings: Int = 0,
  val drums: String = "",
  val bass: String = "",
  val rhythm: String = "",
  val lead: String = "",
  val sort: Sort = Sort.AZ,
) {
  val active: Boolean
    get() =
      name.isNotEmpty() || artist.isNotEmpty() || tuning.isNotEmpty() || bpm.isNotEmpty() || strings != 0 ||
        drums.isNotEmpty() || bass.isNotEmpty() || rhythm.isNotEmpty() || lead.isNotEmpty()
}

@Serializable
data class SearchResult(
  val matches: List<String> = emptyList(), // paths of the matching songs, in list order
  val bpmInvalid: Boolean = false,
  val artists: List<String> = emptyList(), // suggestions for the artist field
  val tunings: List<Tuning> = emptyList(), // suggestions for the tuning field, grouped by string count
)

@Serializable
data class ScanResult(val songs: List<Song> = emptyList(), val summary: String = "", val warning: String? = null)

@Serializable
private enum class Op {
  @SerialName("load") LOAD,
  @SerialName("scan") SCAN,
  @SerialName("search") SEARCH,
}

@Serializable private data class Request(val op: Op, val index: String? = null, val root: String? = null, val query: Query? = null)

/** How long tabscan may take to answer each kind of request before it's stopped. */
data class Timeouts(val searchMs: Long = 10_000, val loadMs: Long = 60_000, val scanMs: Long = 10 * 60_000)

private fun Timeouts.of(op: Op) =
  when (op) {
    Op.SEARCH -> searchMs
    Op.LOAD -> loadMs
    Op.SCAN -> scanMs
  }

@Serializable private data class Response(val error: String? = null)

/** Where the chosen tab folder is remembered. */
interface RootStore {
  var root: String?
}

/** What the screen needs of the tab folder and its search; [Finder] is the real one. */
interface TabSource {
  /** Absolute path of the tab folder; null until one is chosen. */
  val root: String?

  /** Makes [path] the tab folder, dropping the saved scan of the old one. */
  suspend fun chooseRoot(path: String)

  /** The songs of the last scan, sorted by artist and title; none if there was none yet. */
  suspend fun load(): List<Song>

  suspend fun scan(): ScanResult

  suspend fun search(query: Query): SearchResult
}

/**
 * The tab folder and its search, answered by the bundled tabscan running as `tabscan -serve`. It's
 * started once and kept running, so a keystroke's search is a quick round trip.
 *
 * [binary] is the tabscan to run, [dataDir] holds the saved scan (index.jsonl) and tabscan's log, and
 * [store] remembers the folder. On a device these come from the [Context] (see the second constructor); tests
 * pass the host build of tabscan and temp files. A tabscan that doesn't answer within [timeouts] is stopped.
 */
class Finder(private val binary: File, dataDir: File, private val store: RootStore, private val timeouts: Timeouts = Timeouts()) : TabSource {
  /** Android only lets apps execute files from nativeLibraryDir, hence the lib*.so name (built by `mise run bin`). */
  constructor(context: Context) : this(File(context.applicationInfo.nativeLibraryDir, "libtabscan.so"), context.filesDir, SharedPrefsRootStore(context))

  private val index = File(dataDir, "index.jsonl")
  private val log = File(dataDir, "tabscan.log")
  private val json = Companion.json
  private val lock = Mutex()
  private var proc: Process? = null
  private var started = false // a tabscan has been run before
  private lateinit var toGo: BufferedWriter
  private lateinit var fromGo: BufferedReader

  override val root: String?
    get() = store.root

  override suspend fun chooseRoot(path: String) =
    withContext(Dispatchers.IO) {
      store.root = path
      index.delete()
      Unit
    }

  override suspend fun load(): List<Song> = call(Request(Op.LOAD, index = index.path), ScanResult.serializer()).songs

  override suspend fun scan(): ScanResult = call(Request(Op.SCAN, index = index.path, root = checkNotNull(root) { "No tab folder chosen" }), ScanResult.serializer())

  override suspend fun search(query: Query): SearchResult = call(Request(Op.SEARCH, query = query), SearchResult.serializer())

  private suspend fun <T> call(req: Request, answer: KSerializer<T>): T =
    withContext(Dispatchers.IO) {
      lock.withLock {
        val line = json.encodeToString(Request.serializer(), req)
        val reply = runCatching { exchange(req.op, line) }.getOrElse {
          proc?.destroy()
          proc = null
          // tabscan died or hung mid-call: start it again and retry a load or search once. Not a scan:
          // it's slow, and likely what stopped tabscan.
          if (req.op == Op.SCAN) throw it
          exchange(req.op, line)
        }
        json.decodeFromString(Response.serializer(), reply).error?.let { error(it) }
        json.decodeFromString(answer, reply)
      }
    }

  /** Sends [line] (a request of kind [op]) to tabscan, starting it first if needed. */
  private fun exchange(op: Op, line: String): String {
    if (proc?.isAlive != true) {
      val restarted = started
      proc = ProcessBuilder(binary.path, "-serve").redirectError(log).start().also {
        toGo = it.outputStream.bufferedWriter()
        fromGo = it.inputStream.bufferedReader()
      }
      started = true
      // A new process has no songs: give it the saved ones before a search, which relies on them.
      if (restarted && op == Op.SEARCH) roundTrip(Op.LOAD, json.encodeToString(Request.serializer(), Request(Op.LOAD, index = index.path)))
    }
    return roundTrip(op, line)
  }

  /** Sends [line] and reads the answer, stopping tabscan if it takes longer than [op] may. */
  private fun roundTrip(op: Op, line: String): String {
    val p = checkNotNull(proc)
    toGo.write(line)
    toGo.newLine()
    toGo.flush()
    val timedOut = AtomicBoolean()
    val watchdog =
      watchdogs.schedule(
        {
          timedOut.set(true)
          p.destroyForcibly()
        },
        timeouts.of(op),
        TimeUnit.MILLISECONDS,
      )
    try {
      val reply = fromGo.readLine()
      if (timedOut.get()) error("tabscan didn't answer within ${timeouts.of(op) / 1000.0} s")
      return reply ?: error("tabscan stopped: " + (log.readLines().lastOrNull() ?: "no output"))
    } finally {
      watchdog.cancel(false)
    }
  }

  companion object {
    /** Stops tabscans that take too long to answer; one daemon thread for all Finders. */
    private val watchdogs = Executors.newSingleThreadScheduledExecutor { Thread(it, "tabscan-watchdog").apply { isDaemon = true } }

    private const val EXTERNAL_STORAGE = "com.android.externalstorage.documents"

    internal val json = Json {
      ignoreUnknownKeys = true
      coerceInputValues = true // a null list from Go reads as empty
    }

    /**
     * Maps a folder picked with ACTION_OPEN_DOCUMENT_TREE to its file system path, which tabscan
     * needs. Only folders on external storage volumes have one.
     */
    fun treeToPath(uri: Uri): String? =
      if (uri.authority != EXTERNAL_STORAGE) null
      else treeToPath(uri.authority, DocumentsContract.getTreeDocumentId(uri), Environment.getExternalStorageDirectory())

    /** [treeToPath] on the parts of the tree URI: its authority, its document id ("primary:Music/Tabs") and where the primary volume is mounted. */
    internal fun treeToPath(authority: String?, documentId: String, primary: File): String? {
      if (authority != EXTERNAL_STORAGE) return null
      val (volume, path) = documentId.split(":", limit = 2).let { it[0] to it.getOrElse(1) { "" } }
      val base = if (volume == "primary") primary else File("/storage", volume)
      return File(base, path).path
    }
  }
}

/** The folder in the app's SharedPreferences. */
private class SharedPrefsRootStore(context: Context) : RootStore {
  private val prefs = context.getSharedPreferences("settings", Context.MODE_PRIVATE)
  override var root: String?
    get() = prefs.getString("root", null)
    set(value) = prefs.edit { putString("root", value) }
}
