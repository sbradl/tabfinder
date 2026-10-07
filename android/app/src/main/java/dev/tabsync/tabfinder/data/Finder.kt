package dev.tabsync.tabfinder.data

import android.content.Context
import android.net.Uri
import android.os.Environment
import android.provider.DocumentsContract
import androidx.core.content.edit
import java.io.BufferedReader
import java.io.BufferedWriter
import java.io.File
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.KSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

// The JSON of `tabscan -serve` (cmd/tabscan/serve.go). All search logic lives there, in Go,
// shared with the desktop app; these are just its answers.

/** A tuning as shown: "Drop C" on a 6-string; [detail] is its notes unless the label is the notes. */
@Serializable data class Tuning(val strings: Int, val name: String, val notes: String, val label: String, val detail: String = "")

@Serializable
data class Song(
  val path: String, // relative to the tab folder
  val title: String,
  val artist: String,
  val album: String,
  val tunings: List<Tuning> = emptyList(), // defaults let coerceInputValues read a null as empty
  val bpms: List<String> = emptyList(), // distinct tempos in order, the opening one first
  val unreadable: Boolean,
  val openAs: String, // the file name to hand TuxGuitar a copy under
)

/** The filter fields as typed. [strings] is set by picking a tuning suggestion. */
@Serializable
data class Query(val name: String = "", val artist: String = "", val tuning: String = "", val bpm: String = "", val strings: Int = 0) {
  val active: Boolean
    get() = name.isNotEmpty() || artist.isNotEmpty() || tuning.isNotEmpty() || bpm.isNotEmpty() || strings != 0
}

@Serializable
data class SearchResult(
  val matches: List<Int> = emptyList(), // indices into the songs
  val bpmInvalid: Boolean = false,
  val artists: List<String> = emptyList(), // suggestions for the artist field
  val tunings: List<Tuning> = emptyList(), // suggestions for the tuning field, grouped by string count
)

@Serializable
data class ScanResult(val songs: List<Song> = emptyList(), val unreadable: Int = 0, val warning: String? = null)

@Serializable private data class Request(val op: String, val index: String? = null, val root: String? = null, val query: Query? = null)

@Serializable private data class Response(val error: String? = null)

/** Where the chosen tab folder is remembered. */
interface RootStore {
  var root: String?
}

/** What the screen needs of the tab folder and its search; [Finder] is the real one. */
interface TabSource {
  /** Absolute path of the tab folder; setting it drops the saved scan. */
  var root: String?

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
 * pass the host build of tabscan and temp files.
 */
class Finder(private val binary: File, dataDir: File, private val store: RootStore) : TabSource {
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

  override var root: String?
    get() = store.root
    set(value) {
      store.root = value
      index.delete()
    }

  override suspend fun load(): List<Song> = call(Request("load", index = index.path), ScanResult.serializer()).songs

  override suspend fun scan(): ScanResult = call(Request("scan", index = index.path, root = checkNotNull(root) { "No tab folder chosen" }), ScanResult.serializer())

  override suspend fun search(query: Query): SearchResult = call(Request("search", query = query), SearchResult.serializer())

  private suspend fun <T> call(req: Request, answer: KSerializer<T>): T =
    withContext(Dispatchers.IO) {
      lock.withLock {
        val line = json.encodeToString(Request.serializer(), req)
        val reply = runCatching { exchange(req.op, line) }.getOrElse {
          // tabscan died mid-call: start it again and retry once.
          proc?.destroy()
          proc = null
          exchange(req.op, line)
        }
        json.decodeFromString(Response.serializer(), reply).error?.let { error(it) }
        json.decodeFromString(answer, reply)
      }
    }

  /** Sends [line] (a request of kind [op]) to tabscan, starting it first if needed. */
  private fun exchange(op: String, line: String): String {
    if (proc?.isAlive != true) {
      val restarted = started
      proc = ProcessBuilder(binary.path, "-serve").redirectError(log).start().also {
        toGo = it.outputStream.bufferedWriter()
        fromGo = it.inputStream.bufferedReader()
      }
      started = true
      // A new process has no songs: give it the saved ones before a search, which relies on them.
      if (restarted && op == "search") roundTrip(json.encodeToString(Request.serializer(), Request("load", index = index.path)))
    }
    return roundTrip(line)
  }

  private fun roundTrip(line: String): String {
    toGo.write(line)
    toGo.newLine()
    toGo.flush()
    return fromGo.readLine() ?: error("tabscan stopped: " + (log.readLines().lastOrNull() ?: "no output"))
  }

  companion object {
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
