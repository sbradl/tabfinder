package dev.tabsync.tabfinder

import android.app.Activity
import android.app.Instrumentation
import android.content.Intent
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.test.espresso.intent.Intents
import androidx.test.espresso.intent.matcher.IntentMatchers.hasPackage
import java.io.File
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * What only a device can check, end to end (the rest of E-AND-02 to E-AND-11 is in the JVM tests, MainScreenTest
 * and TuxGuitarTest, and the matching in cmd/tabscan): the tabscan bundled in the APK scans a folder on the
 * device's storage and answers a search, a song goes to TuxGuitar through the FileProvider, and a restart shows
 * the saved scan without scanning again.
 */
class SmokeTest : DeviceTest() {
  @Test
  fun scanSearchOpenAndRestart() {
    // E-AND-02: the scan, by the bundled tabscan running as the app's child, saved.
    fixture()
    launch()
    waitForSongs()
    assertEquals("${Fixture.SONGS} / ${Fixture.SONGS}", counter())
    assertEquals(1, tabscanPids().size)
    assertEquals(Fixture.SONGS, index.readLines().count { it.isNotBlank() })

    // E-AND-04: a search answered by it.
    compose.onNodeWithTag("field-tuning").performTextInput("drop c")
    until { counter() == "2 / ${Fixture.SONGS}" }
    compose.onNodeWithTag("clear-tuning").performClick()
    until { counter() == "${Fixture.SONGS} / ${Fixture.SONGS}" }

    // E-AND-07: TuxGuitar gets a copy under the song's real name, readable through the FileProvider.
    Intents.init()
    try {
      Intents.intending(hasPackage(TuxGuitar.PACKAGE)).respondWith(Instrumentation.ActivityResult(Activity.RESULT_OK, null))
      compose.onNodeWithTag("song-Gorsewick/Gravel Hymns/Quartz.gpx.crdownload").performClick()
      until { Intents.getIntents().any { it.`package` == TuxGuitar.PACKAGE } } // the copy is made off the main thread first
      // The recorded intent, not Intents.intended: that waits for the app's window to have focus, which the
      // emulator's background apps sometimes take.
      val intent = Intents.getIntents().single { it.`package` == TuxGuitar.PACKAGE }
      assertEquals(Intent.ACTION_VIEW, intent.action)
      assertTrue("grants read access", intent.flags and Intent.FLAG_GRANT_READ_URI_PERMISSION != 0)
      assertEquals("Quartz.gp3", intent.data!!.lastPathSegment!!.substringAfterLast('/'))
      val original = File(fixtureRoot, "Gorsewick/Gravel Hymns/Quartz.gpx.crdownload").readBytes()
      assertArrayEquals(original, context.contentResolver.openInputStream(intent.data!!)!!.use { it.readBytes() })
    } finally {
      Intents.release()
    }

    // E-AND-03: a restart lists the saved scan without scanning.
    scenario!!.close()
    scenario = null
    killTabscan()
    val saved = index.lastModified()
    Thread.sleep(1100) // a scan would change the time
    launch()
    waitForSongs()
    assertEquals(saved, index.lastModified())
  }
}
