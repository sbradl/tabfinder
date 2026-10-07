package dev.tabsync.tabfinder

import androidx.compose.ui.test.assert
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** E-AND-04 to E-AND-06, the same cases as the desktop app's E-DSK-10 to E-DSK-19, through the Go backend. */
class FilterAndSuggestionTest : DeviceTest() {
  private val total = Fixture.SONGS

  private fun start() {
    fixture()
    launch()
    waitForSongs()
  }

  private fun type(field: String, text: String) = compose.onNodeWithTag("field-$field").performTextInput(text)

  private fun waitForCounter(want: String) = until(10_000) { counter() == want }

  @Test
  fun rowsShowWhatTabscanWorkedOut() {
    start()
    // Subtitle and tempo come from Go (internal/rows), the same as on the desktop.
    compose.onNodeWithTag("song-Amber Marsh/Tide of Lanterns/First Frost.gp3")
      .assert(hasText("Amber Marsh · Tide of Lanterns"))
      .assert(hasText("120"))
      .assert(hasText("BPM"))
  }

  // E-AND-04
  @Test
  fun eachFieldAloneNarrowsTheList() {
    start()
    for ((field, text, matches) in listOf(Triple("artist", "inkwell flamingos", 2), Triple("song", "quartz", 1), Triple("tuning", "drop c", 2), Triple("bpm", "100-110", 2))) {
      type(field, text)
      waitForCounter("$matches / $total")
      compose.onNodeWithTag("clear-$field").performClick()
      waitForCounter("$total / $total")
    }
  }

  @Test
  fun allFieldsCombinedAndCleared() {
    start()
    type("artist", "inkwell flamingos")
    type("tuning", "drop c")
    type("bpm", "100-150")
    waitForCounter("1 / $total")
    compose.onNodeWithTag("clear-tuning").performClick()
    waitForCounter("2 / $total")
  }

  @Test
  fun invalidBpmIsIgnoredAndNoMatchesCanBeCleared() {
    start()
    type("bpm", "fast")
    waitForCounter("$total / $total") // the invalid range filters nothing
    compose.onNodeWithTag("clear-bpm").performClick()
    type("song", "zzzzz")
    until(10_000) { compose.onAllNodes(hasTestTag("prompt")).fetchSemanticsNodes().isNotEmpty() }
    compose.onNodeWithTag("prompt-action").performClick() // "Clear filters"
    waitForCounter("$total / $total")
    assertEquals("", fieldText("song"))
  }

  private fun fieldText(field: String): String =
    compose.onNodeWithTag("field-$field").fetchSemanticsNode().config[androidx.compose.ui.semantics.SemanticsProperties.EditableText].text

  // E-AND-05
  @Test
  fun artistSuggestionsListEachArtistOnce() {
    start()
    compose.onNodeWithTag("field-artist").performClick()
    type("artist", "i")
    until(10_000) { compose.onAllNodes(hasTestTag("suggestion-artist-Inkwell Flamingos")).fetchSemanticsNodes().isNotEmpty() || compose.onAllNodes(hasTestTag("suggestion-artist-INKWELL FLAMINGOS")).fetchSemanticsNodes().isNotEmpty() }
    val spellings = listOf("Inkwell Flamingos", "INKWELL FLAMINGOS").count { compose.onAllNodes(hasTestTag("suggestion-artist-$it")).fetchSemanticsNodes().isNotEmpty() }
    assertEquals("the case variants are one suggestion", 1, spellings)
  }

  @Test
  fun pickingAnArtistFiltersAndTrimsTheTunings() {
    start()
    type("artist", "amber")
    until(10_000) { compose.onAllNodes(hasTestTag("suggestion-artist-Amber Marsh")).fetchSemanticsNodes().isNotEmpty() }
    compose.onNodeWithTag("suggestion-artist-Amber Marsh").performClick()
    waitForCounter("2 / $total")
    type("tuning", "")
    compose.onNodeWithTag("field-tuning").performClick()
    type("tuning", "e")
    until(10_000) { compose.onAllNodes(hasTestTag("section-6 STRINGS")).fetchSemanticsNodes().isNotEmpty() }
    // Amber Marsh plays 6 strings only: no other string count has a header.
    for (n in listOf(4, 5, 7, 8, 9)) assertTrue("section $n shown", compose.onAllNodes(hasTestTag("section-$n STRINGS")).fetchSemanticsNodes().isEmpty())
  }

  @Test
  fun pickingATuningSetsItsStringCount() {
    start()
    compose.onNodeWithTag("field-tuning").performClick()
    type("tuning", "b")
    until(10_000) { compose.onAllNodes(hasTestTag("section-7 STRINGS")).fetchSemanticsNodes().isNotEmpty() }
    compose.onNodeWithTag("suggestion-tuning-B Standard").performClick()
    waitForCounter("2 / $total") // the two 7 string tabs
    // Typing in the field afterwards drops the string count: "B Standard" + "x" matches nothing anyway,
    // deleting it again must show every tuning of that name, on any string count.
    type("tuning", "x")
    compose.onNodeWithTag("field-tuning").performTextClearance()
    waitForCounter("$total / $total")
  }

  // E-AND-06: regression for the "Amber Marshamber" observation, which could not be reproduced.
  @Test
  fun fastTypingKeepsEveryCharacter() {
    start()
    val typed = "amber marshamber mar"
    assertEquals(20, typed.length)
    for (c in typed) compose.onNodeWithTag("field-artist").performTextInput(c.toString())
    compose.waitForIdle()
    assertEquals(typed, fieldText("artist"))
    // The result is the one of the last query, not of an earlier one.
    waitForCounter("0 / $total")
    compose.onNodeWithTag("field-artist").performTextClearance()
    for (c in "soilbed quartet") compose.onNodeWithTag("field-artist").performTextInput(c.toString())
    assertEquals("soilbed quartet", fieldText("artist"))
    waitForCounter("1 / $total")
    assertTrue(compose.onAllNodesWithTag("song-Soilbed Quartet/Glass Orchard/Brass Kettle.gp3").fetchSemanticsNodes().isNotEmpty())
  }
}
