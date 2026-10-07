package dev.tabsync.tabfinder.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.StateRestorationTester
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.printToLog
import androidx.test.ext.junit.runners.AndroidJUnit4
import dev.tabsync.tabfinder.Fixture
import dev.tabsync.tabfinder.MemoryRootStore
import dev.tabsync.tabfinder.data.Finder
import dev.tabsync.tabfinder.data.Song
import dev.tabsync.tabfinder.hostTabscan
import dev.tabsync.tabfinder.theme.TabFinderTheme
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.annotation.Config

/**
 * The screen over the real view model, Finder and host tabscan, scanning the made-up library: what the
 * device tests E-AND-01 and E-AND-03 to E-AND-11 check of the screen. What only a device can check (storage
 * access, the bundled tabscan, TuxGuitar's intent) is in androidTest.
 */
@RunWith(AndroidJUnit4::class)
@Config(qualifiers = "w800dp-h1280dp-port-hdpi") // the tablet's screen
class MainScreenTest {
  @get:Rule val compose = createComposeRule()
  @get:Rule val folder = TemporaryFolder()

  private val total = Fixture.SONGS
  private val library: File by lazy {
    folder.newFolder("Tabs").also { root -> for ((path, data) in Fixture.library) File(root, path).apply { parentFile!!.mkdirs() }.writeBytes(data) }
  }
  private val data: File by lazy { folder.newFolder("data") }
  private val opened = mutableListOf<Song>()
  private var openAnswer: String? = null
  private var folderPicks = 0
  private var accessRequests = 0

  private fun viewModel(root: String? = library.path) = MainViewModel(Finder(hostTabscan(), data, MemoryRootStore(root)))

  private fun show(viewModel: MainViewModel = viewModel(), hasAccess: Boolean = true) {
    compose.setContent { Screen(viewModel, hasAccess) }
  }

  @androidx.compose.runtime.Composable
  private fun Screen(viewModel: MainViewModel, hasAccess: Boolean = true) =
    TabFinderTheme {
      MainScreen(
        viewModel,
        onPickFolder = { folderPicks++ },
        onOpen = { _, song -> opened += song; openAnswer },
        hasAccess = hasAccess,
        onRequestAccess = { accessRequests++ },
      )
    }

  private fun start() {
    show()
    waitForCounter("$total / $total")
  }

  private fun until(cond: () -> Boolean) =
    try {
      compose.waitUntil(10_000) { runCatching(cond).getOrDefault(false) }
    } catch (e: androidx.compose.ui.test.ComposeTimeoutException) {
      compose.onRoot().printToLog("SCREEN") // what was on screen
      throw e
    }

  private fun counter(): String =
    compose.onAllNodesWithTag("counter").fetchSemanticsNodes().firstOrNull()?.config?.getOrNull(SemanticsProperties.Text)?.joinToString("")?.trim() ?: ""

  private fun waitForCounter(want: String) = until { counter() == want }

  private fun shown(tag: String) = compose.onAllNodes(hasTestTag(tag)).fetchSemanticsNodes().isNotEmpty()

  private fun type(field: String, text: String) = compose.onNodeWithTag("field-$field").performTextInput(text)

  private fun fieldText(field: String): String = compose.onNodeWithTag("field-$field").fetchSemanticsNode().config[SemanticsProperties.EditableText].text

  // E-AND-01: the prompts before the list.
  @Test
  fun withoutAccessTheScreenAsksForIt() {
    show(viewModel(), hasAccess = false)
    compose.onNodeWithText("Allow file access").assertExists()
    compose.onNodeWithTag("prompt-action").performClick()
    assertEquals(1, accessRequests)
  }

  @Test
  fun withoutAFolderTheScreenAsksForOne() {
    show(viewModel(root = null))
    compose.onNodeWithText("Choose your tab folder").assertExists()
    compose.onNodeWithTag("prompt-action").performClick()
    assertEquals(1, folderPicks)
  }

  // E-AND-02: the list of a scan, with what tabscan worked out (subtitle and tempo come from Go's internal/rows).
  @Test
  fun aScanFillsTheList() {
    start()
    compose.onNodeWithTag("song-Amber Marsh/Tide of Lanterns/First Frost.gp3")
      .assert(hasText("Amber Marsh · Tide of Lanterns"))
      .assert(hasText("120"))
      .assert(hasText("BPM"))
    assertEquals(total, File(data, "index.jsonl").readLines().count { it.isNotBlank() })
  }

  // E-AND-04 (the matching itself: cmd/tabscan TestServeAppFilters)
  @Test
  fun eachFieldAloneNarrowsTheListAndClears() {
    start()
    for ((field, text, matches) in listOf(Triple("artist", "inkwell flamingos", 2), Triple("song", "quartz", 1), Triple("tuning", "drop c", 2), Triple("bpm", "100-110", 2))) {
      type(field, text)
      waitForCounter("$matches / $total")
      compose.onNodeWithTag("clear-$field").performClick()
      waitForCounter("$total / $total")
    }
  }

  @Test
  fun allFieldsCombine() {
    start()
    type("artist", "inkwell flamingos")
    type("tuning", "drop c")
    type("bpm", "100-150")
    waitForCounter("1 / $total")
    compose.onNodeWithTag("clear-tuning").performClick()
    waitForCounter("2 / $total")
  }

  @Test
  fun noMatchesCanBeCleared() {
    start()
    type("bpm", "fast")
    waitForCounter("$total / $total") // an invalid range filters nothing
    type("song", "zzzzz")
    until { shown("prompt") }
    compose.onNodeWithTag("prompt-action").performClick() // "Clear filters"
    waitForCounter("$total / $total")
    assertEquals("", fieldText("song"))
    assertEquals("", fieldText("bpm"))
  }

  // E-AND-05 (the suggestions themselves: cmd/tabscan TestServeAppSuggestions)
  @Test
  fun pickingAnArtistFillsTheFieldAndTrimsTheTunings() {
    start()
    type("artist", "amber")
    until { shown("suggestion-artist-Amber Marsh") }
    compose.onNodeWithTag("suggestion-artist-Amber Marsh").performClick()
    waitForCounter("2 / $total")
    assertEquals("Amber Marsh", fieldText("artist"))
    compose.onNodeWithTag("field-tuning").performClick()
    type("tuning", "e")
    until { shown("section-6 STRINGS") }
    for (n in listOf(4, 5, 7, 8, 9)) assertTrue("section $n shown", !shown("section-$n STRINGS"))
  }

  @Test
  fun pickingATuningSetsItsStringCountUntilTheFieldIsTyped() {
    start()
    compose.onNodeWithTag("field-tuning").performClick()
    type("tuning", "b")
    until { shown("section-7 STRINGS") }
    compose.onNodeWithTag("suggestion-tuning-B Standard").performClick()
    waitForCounter("2 / $total") // the two 7 string tabs
    type("tuning", "x")
    waitForCounter("0 / $total")
    compose.onNodeWithTag("field-tuning").performTextClearance()
    waitForCounter("$total / $total")
  }

  // E-AND-06: regression for the "Amber Marshamber" observation.
  @Test
  fun fastTypingKeepsEveryCharacter() {
    start()
    val typed = "amber marshamber mar"
    for (c in typed) type("artist", c.toString())
    assertEquals(typed, fieldText("artist"))
    waitForCounter("0 / $total") // the result of the last query, not of an earlier one
    compose.onNodeWithTag("field-artist").performTextClearance()
    for (c in "soilbed quartet") type("artist", c.toString())
    waitForCounter("1 / $total")
    assertTrue(shown("song-Soilbed Quartet/Glass Orchard/Brass Kettle.gp3"))
  }

  // E-AND-07, E-AND-08: tapping a song opens it; why it couldn't be opened is shown (TuxGuitarTest has the reasons).
  @Test
  fun tappingASongOpensItAndShowsWhyItCouldNot() {
    start()
    compose.onNodeWithTag("song-Gorsewick/Gravel Hymns/Quartz.gpx.crdownload").performClick()
    until { opened.isNotEmpty() }
    assertEquals("Quartz.gp3", opened.single().openAs)
    openAnswer = "TuxGuitar is not installed"
    compose.onNodeWithTag("song-Gorsewick/Gravel Hymns/Quartz.gpx.crdownload").performClick()
    until { compose.onAllNodes(hasText("TuxGuitar is not installed")).fetchSemanticsNodes().isNotEmpty() }
  }

  // E-AND-10: a rotation keeps the view model; the screen's own state comes back from the saved state.
  @Test
  fun aRotationKeepsTheListAndTheFilters() {
    val restoration = StateRestorationTester(compose)
    val vm = viewModel()
    restoration.setContent { Screen(vm) }
    waitForCounter("$total / $total")
    type("artist", "inkwell flamingos")
    waitForCounter("2 / $total")
    restoration.emulateSavedInstanceStateRestore()
    waitForCounter("2 / $total")
    assertEquals("inkwell flamingos", fieldText("artist"))
  }

  // E-AND-03, E-AND-10: after process death the activity comes back with a new view model: the list is the
  // saved scan, without scanning again (the filters aren't saved).
  @Test
  fun afterProcessDeathTheSavedScanIsBackWithoutScanning() {
    val restoration = StateRestorationTester(compose)
    var vm by mutableStateOf(viewModel())
    restoration.setContent { Screen(vm) }
    waitForCounter("$total / $total")
    val index = File(data, "index.jsonl")
    val saved = index.lastModified()
    Thread.sleep(1100) // a rescan would change the time
    vm = viewModel()
    restoration.emulateSavedInstanceStateRestore()
    waitForCounter("$total / $total")
    assertEquals(saved, index.lastModified())
  }

  // E-AND-11 (the picker's answer: FolderPickTest)
  @Test
  fun anotherFolderReplacesTheList() {
    val vm = viewModel()
    show(vm)
    waitForCounter("$total / $total")
    compose.onNodeWithContentDescription("Choose tab folder").performClick()
    assertEquals(1, folderPicks)
    val other = folder.newFolder("Other")
    File(other, "Other Band/Other Album/Other Song.gp3").apply { parentFile!!.mkdirs() }.writeBytes(Fixture.library.getValue("Amber Marsh/Tide of Lanterns/First Frost.gp3"))
    vm.setRoot(other.path)
    waitForCounter("1 / 1")
    assertTrue(shown("song-Other Band/Other Album/Other Song.gp3"))
    assertEquals(1, File(data, "index.jsonl").readLines().count { it.isNotBlank() })
  }
}
