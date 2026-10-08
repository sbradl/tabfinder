package dev.tabsync.tabfinder.ui

import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.printToLog
import androidx.test.ext.junit.runners.AndroidJUnit4
import dev.tabsync.tabfinder.MemoryRootStore
import dev.tabsync.tabfinder.data.Finder
import dev.tabsync.tabfinder.hostTabscan
import dev.tabsync.tabfinder.theme.TabFinderTheme
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.annotation.Config

/**
 * The difficulty of the parts on the screen (E-AND-13 to E-AND-15), over the real view model, Finder and host
 * tabscan, with a saved scan of made-up songs with parts: Guitar Pro files with notes would give the same.
 */
@RunWith(AndroidJUnit4::class)
@Config(qualifiers = "w800dp-h1280dp-port-hdpi") // the tablet's screen
class DifficultyScreenTest {
  @get:Rule val compose = createComposeRule()
  @get:Rule val folder = TemporaryFolder()

  private fun song(title: String, vararg parts: String) =
    """{"path":"Copper Wolves/$title.gp5","format":"gp5","artist":"Copper Wolves","title":"$title","parts":[${parts.joinToString(",")}]}"""

  private fun part(role: String, score: Double) = """{"role":"$role","tracks":[0],"score":$score}"""

  /** [more] songs after the four, so the list scrolls: "Filler 01"... with a rhythm part of level 3. */
  private fun start(more: Int = 0) {
    val data = folder.newFolder("data")
    File(data, "index.jsonl").writeText(
      (listOf(
          """{"tabfinderIndex":3}""",
          song("Anvil", part("drums", 2.2), part("rhythm", 4.6)),
          song("Bellows", part("drums", 7.8), part("bass", 5.0), part("rhythm", 6.4), part("lead", 9.1)),
          song("Cinder", part("rhythm", 7.0)),
          """{"path":"Copper Wolves/Dross.tg","format":"tg","artist":"Copper Wolves","title":"Dross"}""",
        ) + (1..more).map { song("Filler %02d".format(it), part("rhythm", 3.0)) })
        .joinToString("\n", postfix = "\n")
    )
    if (more > 0) {
      val vm = MainViewModel(Finder(hostTabscan(), data, MemoryRootStore(folder.newFolder("Tabs").path)))
      compose.setContent { TabFinderTheme { MainScreen(vm, onPickFolder = {}, onOpen = { _, _ -> null }) } }
      until { titles().firstOrNull() == "Anvil" }
      return
    }
    val vm = MainViewModel(Finder(hostTabscan(), data, MemoryRootStore(folder.newFolder("Tabs").path)))
    compose.setContent { TabFinderTheme { MainScreen(vm, onPickFolder = {}, onOpen = { _, _ -> null }) } }
    waitForTitles(listOf("Anvil", "Bellows", "Cinder", "Dross"))
  }

  private fun until(cond: () -> Boolean) =
    try {
      compose.waitUntil(10_000) { runCatching(cond).getOrDefault(false) }
    } catch (e: androidx.compose.ui.test.ComposeTimeoutException) {
      compose.onRoot().printToLog("SCREEN")
      throw e
    }

  /** The titles of the rows, top to bottom. */
  private fun titles(): List<String> =
    compose
      .onAllNodes(SemanticsMatcher("a song row") { it.config.getOrNull(SemanticsProperties.TestTag)?.startsWith("song-") == true })
      .fetchSemanticsNodes()
      .map { it.config[SemanticsProperties.TestTag].removePrefix("song-Copper Wolves/").substringBeforeLast('.') }

  private fun waitForTitles(want: List<String>) = until { titles() == want }

  private fun shown(tag: String) = compose.onAllNodes(hasTestTag(tag)).fetchSemanticsNodes().isNotEmpty()

  /** Moves the thumbs of a role's slider in the difficulty dialog. */
  private fun slide(role: String, from: Int, to: Int) {
    val thumbs = compose.onAllNodes(SemanticsMatcher.keyIsDefined(SemanticsActions.SetProgress) and hasAnyAncestor(hasTestTag("slider-$role")))
    // The high end first, so the low one never has to pass it.
    thumbs[1].performSemanticsAction(SemanticsActions.SetProgress) { it(to.toFloat()) }
    thumbs[0].performSemanticsAction(SemanticsActions.SetProgress) { it(from.toFloat()) }
    compose.waitForIdle()
  }

  // E-AND-13
  @Test
  fun rowsShowTheirParts() {
    start()
    compose.onNodeWithTag("part-Copper Wolves/Bellows.gp5-rhythm").assert(hasText("6"))
    compose.onNodeWithTag("part-Copper Wolves/Bellows.gp5-lead").assert(hasText("9"))
    compose.onNodeWithTag("part-Copper Wolves/Anvil.gp5-drums").assert(hasText("2"))
    assertEquals(4, compose.onAllNodes(SemanticsMatcher("a part of Bellows") { it.config.getOrNull(SemanticsProperties.TestTag)?.startsWith("part-Copper Wolves/Bellows.gp5-") == true }).fetchSemanticsNodes().size)
    assertEquals(false, shown("part-Copper Wolves/Dross.tg-rhythm"))
  }

  // E-AND-14
  @Test
  fun aRangeOfLevelsPerPartNarrowsTheListAndShowsAsAChip() {
    start()
    compose.onNodeWithTag("difficulty").performClick()
    compose.onNodeWithTag("difficulty-dialog").assertIsDisplayed()
    for (role in listOf("drums", "bass", "rhythm", "lead")) compose.onNodeWithTag("slider-$role").assertIsDisplayed()
    slide("rhythm", 6, 7)
    waitForTitles(listOf("Bellows", "Cinder")) // while the dialog is open
    compose.onNodeWithTag("difficulty-done").performClick()
    until { !shown("difficulty-dialog") }
    compose.onNodeWithTag("chip-rhythm").assert(hasText("6–7"))
    // Easy drums: songs with drums only.
    compose.onNodeWithTag("difficulty").performClick()
    slide("drums", 1, 3)
    compose.onNodeWithTag("difficulty-done").performClick()
    until { titles().isEmpty() } // Anvil's rhythm is 5
    // Its ✕ drops one range; Reset in the dialog drops them all.
    compose.onNodeWithTag("chip-clear-rhythm").performClick()
    waitForTitles(listOf("Anvil"))
    assertEquals(false, shown("chip-rhythm"))
    compose.onNodeWithTag("difficulty").performClick()
    compose.onNodeWithTag("difficulty-reset").performClick()
    compose.onNodeWithTag("difficulty-done").performClick()
    waitForTitles(listOf("Anvil", "Bellows", "Cinder", "Dross"))
    assertEquals(false, shown("chip-drums"))
  }

  // E-AND-15
  @Test
  fun theSortMenuOrdersTheList() {
    start()
    fun sortBy(item: String) {
      compose.onNodeWithContentDescription("Sort").performClick()
      compose.onNodeWithTag("sort-$item").performClick()
    }
    sortBy("hardest")
    waitForTitles(listOf("Bellows", "Cinder", "Anvil", "Dross"))
    compose.onNodeWithTag("sort-button").assert(hasText("Hardest"))
    sortBy("easiest")
    waitForTitles(listOf("Anvil", "Cinder", "Bellows", "Dross"))
    sortBy("az")
    waitForTitles(listOf("Anvil", "Bellows", "Cinder", "Dross"))
    compose.onNodeWithTag("sort-button").assert(hasText("A–Z"))
  }

  // E-AND-15: a new order starts at the top, even where the list scrolls.
  @Test
  fun aNewOrderStartsAtTheTop() {
    start(more = 30)
    compose.onNodeWithContentDescription("Sort").performClick()
    compose.onNodeWithTag("sort-hardest").performClick()
    until { titles().firstOrNull() == "Bellows" }
    compose.waitForIdle()
    val list = compose.onNodeWithTag("songs").fetchSemanticsNode().boundsInRoot
    val first = compose.onNodeWithTag("song-Copper Wolves/Bellows.gp5").fetchSemanticsNode().boundsInRoot
    assertEquals("the hardest song's row starts at the top of the list", list.top, first.top, 0.5f)
  }

  // E-AND-14: so does a list a filter changed: dropping one brings back the songs before.
  @Test
  fun aChangedFilterStartsAtTheTop() {
    start(more = 30)
    compose.onNodeWithTag("difficulty").performClick()
    slide("rhythm", 7, 7)
    compose.onNodeWithTag("difficulty-done").performClick()
    until { titles() == listOf("Cinder") }
    compose.onNodeWithTag("chip-clear-rhythm").performClick()
    until { titles().firstOrNull() == "Anvil" }
    compose.waitForIdle()
    val list = compose.onNodeWithTag("songs").fetchSemanticsNode().boundsInRoot
    val first = compose.onNodeWithTag("song-Copper Wolves/Anvil.gp5").fetchSemanticsNode().boundsInRoot
    assertEquals("the first song's row starts at the top of the list", list.top, first.top, 0.5f)
  }
}
