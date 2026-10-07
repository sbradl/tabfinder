package dev.tabsync.tabfinder

import android.app.Activity
import android.app.Instrumentation
import android.content.Intent
import android.content.pm.ActivityInfo
import android.content.pm.PackageManager
import android.provider.DocumentsContract
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.test.espresso.intent.Intents
import androidx.test.espresso.intent.matcher.IntentMatchers.hasAction
import androidx.test.espresso.intent.matcher.IntentMatchers.hasFlag
import androidx.test.espresso.intent.matcher.IntentMatchers.hasPackage
import java.io.File
import org.hamcrest.Matchers.allOf
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** E-AND-07 to E-AND-11 */
class OpenAndLifecycleTest : DeviceTest() {
  private fun start() {
    fixture()
    launch()
    waitForSongs()
  }

  private fun tuxGuitarInstalled() =
    try {
      context.packageManager.getPackageInfo(TuxGuitar.PACKAGE, 0)
      true
    } catch (e: PackageManager.NameNotFoundException) {
      false
    }

  // E-AND-07
  @Test
  fun tappingASongHandsTuxGuitarACopyUnderItsRealName() {
    start()
    Intents.init()
    try {
      Intents.intending(hasPackage(TuxGuitar.PACKAGE)).respondWith(Instrumentation.ActivityResult(Activity.RESULT_OK, null))
      compose.onNodeWithTag("song-Gorsewick/Gravel Hymns/Quartz.gpx.crdownload").performClick()
      until { Intents.getIntents().any { it.`package` == TuxGuitar.PACKAGE } } // the copy is made off the main thread first
      Intents.intended(allOf(hasAction(Intent.ACTION_VIEW), hasPackage(TuxGuitar.PACKAGE), hasFlag(Intent.FLAG_GRANT_READ_URI_PERMISSION)))
      val intent = Intents.getIntents().single { it.`package` == TuxGuitar.PACKAGE }
      val uri = intent.data!!
      assertEquals("content", uri.scheme)
      assertEquals("${context.packageName}.files", uri.authority)
      assertEquals("Quartz.gp3", uri.lastPathSegment!!.substringAfterLast('/'))
      val original = File(fixtureRoot, "Gorsewick/Gravel Hymns/Quartz.gpx.crdownload").readBytes()
      assertArrayEquals(original, context.contentResolver.openInputStream(uri)!!.use { it.readBytes() })
    } finally {
      Intents.release()
    }
  }

  // Without TuxGuitar: if it is installed, it is disabled for the test (which makes its intents unresolvable,
  // as if it were gone) and enabled again after, also when the test fails.
  @Test
  fun withoutTuxGuitarTheAppSaysSo() {
    val wasEnabled = tuxGuitarInstalled() && shell("pm list packages -e ${TuxGuitar.PACKAGE}").contains(TuxGuitar.PACKAGE)
    if (wasEnabled) shell("pm disable-user --user 0 ${TuxGuitar.PACKAGE}")
    try {
      start()
      compose.onNodeWithTag("song-Soilbed Quartet/Glass Orchard/Brass Kettle.gp3").performClick()
      until(10_000) { compose.onAllNodes(hasText("TuxGuitar is not installed")).fetchSemanticsNodes().isNotEmpty() }
    } finally {
      if (wasEnabled) shell("pm enable ${TuxGuitar.PACKAGE}")
    }
  }

  // E-AND-08
  @Test
  fun aSongDeletedAfterTheScanShowsAnError() {
    start()
    Intents.init()
    try {
      Intents.intending(hasPackage(TuxGuitar.PACKAGE)).respondWith(Instrumentation.ActivityResult(Activity.RESULT_OK, null))
      File(fixtureRoot, "Soilbed Quartet/Glass Orchard/Brass Kettle.gp3").delete()
      compose.onNodeWithTag("song-Soilbed Quartet/Glass Orchard/Brass Kettle.gp3").performClick()
      until(10_000) { compose.onAllNodes(hasText("Brass Kettle.gp3", substring = true)).fetchSemanticsNodes().isNotEmpty() }
      // Still alive, and still listing.
      assertTrue(songRows().isNotEmpty())
      compose.onNodeWithTag("field-artist").performClick()
    } finally {
      Intents.release()
    }
  }

  // E-AND-09: this is where a killed tabscan must be restarted and given the songs again.
  @Test
  fun aKilledTabscanIsRestartedAndKeepsAnswering() {
    start()
    compose.onNodeWithTag("field-artist").performTextInput("soil")
    until(10_000) { counter() == "1 / ${Fixture.SONGS}" }
    val before = tabscanPids()
    assertEquals(1, before.size)
    killTabscan()
    compose.onNodeWithTag("field-artist").performTextInput("b")
    until(10_000) { counter() == "1 / ${Fixture.SONGS}" }
    compose.onNodeWithTag("field-artist").performTextInput("x") // "soilbx": nothing
    until(10_000) { counter() == "0 / ${Fixture.SONGS}" }
    val after = tabscanPids()
    assertEquals(1, after.size)
    assertFalse("the old process is gone", before.single() == after.single())
  }

  // E-AND-10: a rotation. (Process death can't be done from inside the app's own process:
  // see scripts/device-process-death.sh.)
  @Test
  fun rotationKeepsTheListAndTheFilters() {
    start()
    compose.onNodeWithTag("field-artist").performTextInput("inkwell flamingos")
    until(10_000) { counter() == "2 / ${Fixture.SONGS}" }
    for (orientation in listOf(ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE, ActivityInfo.SCREEN_ORIENTATION_PORTRAIT)) {
      scenario!!.onActivity { it.requestedOrientation = orientation }
      compose.waitForIdle()
      until(10_000) { counter() == "2 / ${Fixture.SONGS}" }
      assertTrue("a row of the filtered list is on screen", songRows().isNotEmpty())
    }
    scenario!!.recreate()
    until(10_000) { counter() == "2 / ${Fixture.SONGS}" }
  }

  // E-AND-11: the picker is stubbed; its answer is a tree URI like the system picker's.
  @Test
  fun choosingAnotherFolderDropsTheOldScanAndScansTheNewOne() {
    start()
    val other = File(fixtureRoot.parentFile, "TabFinderOther")
    fixture(mapOf("Other Band/Other Album/Other Song.gp3" to Fixture.library.getValue("Amber Marsh/Tide of Lanterns/First Frost.gp3")), root = other, choose = false)
    val rel = other.path.removePrefix(android.os.Environment.getExternalStorageDirectory().path + "/")
    val tree = DocumentsContract.buildTreeDocumentUri("com.android.externalstorage.documents", "primary:$rel")
    Intents.init()
    try {
      Intents.intending(hasAction(Intent.ACTION_OPEN_DOCUMENT_TREE)).respondWith(Instrumentation.ActivityResult(Activity.RESULT_OK, Intent().setData(tree)))
      val oldIndex = index.readText()
      compose.onNodeWithContentDescription("Choose tab folder").performClick()
      until(20_000) { counter() == "1 / 1" }
      assertTrue(compose.onAllNodes(hasTestTag("song-Other Band/Other Album/Other Song.gp3")).fetchSemanticsNodes().isNotEmpty())
      assertFalse(index.readText() == oldIndex)
      assertEquals(1, index.readLines().count { it.isNotBlank() })
      assertEquals(other.path, context.getSharedPreferences("settings", 0).getString("root", null))
    } finally {
      Intents.release()
    }
  }
}
