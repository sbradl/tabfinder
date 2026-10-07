package dev.tabsync.tabfinder

import android.Manifest
import android.content.Context
import android.os.Build
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.isRoot
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.printToLog
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.test.core.app.ActivityScenario
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.rule.GrantPermissionRule
import java.io.File
import java.nio.ByteBuffer
import java.nio.ByteOrder
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.rules.TestRule

/**
 * Base of the device tests. Debug builds are a separate app (applicationIdSuffix), so these tests
 * set its folder and scan freely. The fixture folder is in the app's own external storage, which
 * tabscan can read without any permission; the permission is granted anyway, for the app's gate.
 */
abstract class DeviceTest {
  protected val instrumentation = InstrumentationRegistry.getInstrumentation()
  protected val context: Context = instrumentation.targetContext

  /**
   * Set (`-Pandroid.testInstrumentationRunnerArguments.withoutAccess=true`) to run without the storage
   * permission, which only a freshly installed app lacks: the one test of the no-access prompt needs that,
   * the others need the permission. The permission can't be revoked from inside the app, as that kills its process.
   */
  protected val withoutAccess = InstrumentationRegistry.getArguments().getString("withoutAccess") == "true"

  // READ_EXTERNAL_STORAGE is only requested up to Android 10 (granting it later throws); from Android 11 on,
  // access is the MANAGE_EXTERNAL_STORAGE app op set in grantAllFilesAccess.
  @get:Rule(order = 0)
  val permission: TestRule =
    if (withoutAccess || Build.VERSION.SDK_INT > Build.VERSION_CODES.Q) TestRule { base, _ -> base }
    else GrantPermissionRule.grant(Manifest.permission.READ_EXTERNAL_STORAGE)
  @get:Rule(order = 1) val compose = createEmptyComposeRule()

  protected val fixtureRoot = File(context.getExternalFilesDir(null), "TabFinderTest")
  protected val index = File(context.filesDir, "index.jsonl")
  private val prefs = context.getSharedPreferences("settings", Context.MODE_PRIVATE)
  protected var scenario: ActivityScenario<MainActivity>? = null
  private var savedRotation = emptyList<Pair<String, String>>()

  @Before
  fun grantAllFilesAccess() {
    // These tests wipe the app's saved folder and scan: only ever the debug build, next to the real app.
    check(context.packageName.endsWith(".debug")) { "device tests must run against the .debug app, not ${context.packageName}" }
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R && !withoutAccess) shell("appops set ${context.packageName} MANAGE_EXTERNAL_STORAGE allow")
    fixtureRoot.deleteRecursively()
    index.delete()
    prefs.edit().clear().commit()
    // Auto-rotate would turn the screen whenever the tablet is moved mid-test: landscape fits fewer rows
    // and shrinks the suggestion menu. Pin it to portrait; tearDown puts the user's setting back.
    savedRotation = listOf("accelerometer_rotation", "user_rotation").map { it to shell("settings get system $it").trim() }
    shell("settings put system accelerometer_rotation 0")
    shell("settings put system user_rotation 0")
  }

  @After
  fun tearDown() {
    scenario?.close()
    killTabscan()
    for ((key, value) in savedRotation) if (value.isNotEmpty() && value != "null") shell("settings put system $key $value")
    fixtureRoot.deleteRecursively()
    File(fixtureRoot.parentFile, "TabFinderOther").deleteRecursively()
  }

  /** Writes the fixture tab folder, and optionally chooses it as the app's folder. */
  protected fun fixture(files: Map<String, ByteArray> = Fixture.library, root: File = fixtureRoot, choose: Boolean = true): File {
    for ((path, data) in files) File(root, path).apply { parentFile!!.mkdirs() }.writeBytes(data)
    if (choose) prefs.edit().putString("root", root.path).commit()
    return root
  }

  protected fun launch(): ActivityScenario<MainActivity> =
    ActivityScenario.launch(MainActivity::class.java).also {
      scenario = it
      until(15_000) { compose.onAllNodes(isRoot()).fetchSemanticsNodes().isNotEmpty() }
    }

  /** Waits for [cond]; it may throw while the activity has no compose hierarchy yet (starting, recreated). */
  protected fun until(timeoutMs: Long = 10_000, cond: () -> Boolean) =
    try {
      compose.waitUntil(timeoutMs) { runCatching(cond).getOrDefault(false) }
    } catch (e: androidx.compose.ui.test.ComposeTimeoutException) {
      runCatching { compose.onRoot(useUnmergedTree = true).printToLog("DEVTEST") } // what was on screen, for `adb logcat -s DEVTEST`
      throw e
    }

  protected fun waitForSongs(timeoutMs: Long = 20_000) {
    until(timeoutMs) { songRows().isNotEmpty() }
    // The whole scan arrives at once, but let the screen settle before tapping around.
    until(timeoutMs) { counter().let { it.startsWith("${Fixture.SONGS} /") || !it.contains("/") } }
  }

  /** The fixture songs whose rows are on screen. */
  protected fun songRows(): List<String> = Fixture.library.keys.filter { compose.onAllNodesWithTag("song-$it").fetchSemanticsNodes().isNotEmpty() }

  /** The counter without its padding: "3 / 12". */
  protected fun counter(): String =
    compose.onAllNodesWithTag("counter").fetchSemanticsNodes().firstOrNull()?.config?.getOrNull(SemanticsProperties.Text)?.joinToString("")?.trim() ?: "" // empty while the activity is being recreated

  protected fun shell(command: String): String {
    val pfd = instrumentation.uiAutomation.executeShellCommand(command)
    return android.os.ParcelFileDescriptor.AutoCloseInputStream(pfd).use { it.readBytes().decodeToString() }
  }

  /** The pids of the tabscan processes the app started. */
  protected fun tabscanPids(): List<Int> =
    Runtime.getRuntime().exec(arrayOf("sh", "-c", "ps -A -o PID,ARGS")).inputStream.bufferedReader().readLines()
      .filter { it.contains("libtabscan.so") }
      .mapNotNull { it.trim().substringBefore(' ').toIntOrNull() }

  protected fun killTabscan() {
    for (pid in tabscanPids()) Runtime.getRuntime().exec(arrayOf("sh", "-c", "kill -9 $pid")).waitFor()
  }
}

/** Synthetic tab files: Guitar Pro 3 with tunings and a tempo, TuxGuitar 1 headers, junk. */
object Fixture {
  private fun gp3(title: String, artist: String, album: String, tempo: Int, vararg tracks: Pair<String, List<Int>>): ByteArray {
    val b = ByteBuffer.allocate(64 * 1024).order(ByteOrder.LITTLE_ENDIAN)
    fun byteSize(s: String, size: Int) {
      b.put(s.length.toByte())
      b.put(s.toByteArray().copyOf(size))
    }
    fun intByteSize(s: String) {
      b.putInt(s.length + 1)
      byteSize(s, s.length)
    }
    byteSize("FICHIER GUITAR PRO v3.00", 30)
    listOf(title, "", artist, album, "", "", "", "").forEach(::intByteSize)
    b.putInt(0).put(0).putInt(tempo).putInt(0)
    repeat(64) { b.putInt(0).put(ByteArray(8)) }
    b.putInt(0).putInt(tracks.size)
    for ((name, strings) in tracks) {
      b.put(0)
      byteSize(name, 40)
      b.putInt(strings.size)
      for (i in 0 until 7) b.putInt(strings.getOrElse(i) { 0 })
      b.putInt(1).putInt(1).putInt(0).putInt(0).putInt(0).putInt(0)
    }
    return b.array().copyOf(b.position())
  }

  val dropC6 = "G" to listOf(62, 57, 53, 48, 43, 36)
  val eStd6 = "G" to listOf(64, 59, 55, 50, 45, 40)
  val custom6 = "G" to listOf(62, 59, 55, 50, 43, 38)
  val bStd7 = "G7" to listOf(64, 59, 55, 50, 45, 40, 35)
  val bass4 = "B" to listOf(43, 38, 33, 28)

  /** Seven readable tabs and one broken one. */
  val library: Map<String, ByteArray> =
    mapOf(
      "Soilbed Quartet/Glass Orchard/Brass Kettle.gp3" to gp3("Brass Kettle", "Soilbed Quartet", "Glass Orchard", 190, dropC6, bass4),
      "Amber Marsh/Tide of Lanterns/First Frost.gp3" to gp3("First Frost", "Amber Marsh", "Tide of Lanterns", 120, eStd6),
      "Amber Marsh/Tide of Lanterns/Where Rivers Seem to Rest.gp3" to gp3("Where Rivers Seem to Rest", "Amber Marsh", "Tide of Lanterns", 125, custom6),
      "Inkwell Flamingos/Mossman/Mirage.gp3" to gp3("Mirage", "Inkwell Flamingos", "Mossman", 140, dropC6),
      "INKWELL FLAMINGOS/Mossman/Paper Ride.gp3" to gp3("Paper Ride", "INKWELL FLAMINGOS", "Mossman", 100, bStd7),
      "Merrowgate/Catch Fortyone/Compass Lost.gp3" to gp3("Compass Lost", "Merrowgate", "Catch Fortyone", 110, bStd7),
      // Named for something else than its content, so TuxGuitar gets a copy under the right name.
      "Gorsewick/Gravel Hymns/Quartz.gpx.crdownload" to gp3("Quartz", "Gorsewick", "Gravel Hymns", 125, eStd6),
      "Broken/garbled.gp5" to "not a tab".toByteArray(),
    )

  const val SONGS = 8
  const val UNREADABLE = 1
}
