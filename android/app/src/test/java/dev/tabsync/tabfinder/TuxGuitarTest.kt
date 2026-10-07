package dev.tabsync.tabfinder

import android.app.Activity
import android.content.Intent
import androidx.core.content.FileProvider
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import dev.tabsync.tabfinder.data.Song
import java.io.File
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.Shadows.shadowOf

/** E-AND-07, E-AND-08: how a song is handed to TuxGuitar, and why it can't be. The device smoke test hands one over for real. */
@RunWith(AndroidJUnit4::class)
class TuxGuitarTest {
  @get:Rule val folder = TemporaryFolder()

  private val app = ApplicationProvider.getApplicationContext<android.app.Application>()
  private val activity = Robolectric.buildActivity(Activity::class.java).setup().get() // the app opens songs from its activity
  // Named for something else than its content, so TuxGuitar gets a copy under the right name.
  private val song = Song("Gorsewick/Gravel Hymns/Quartz.gpx.crdownload", "Quartz", "Gorsewick", "", emptyList(), false, "Quartz.gp3")
  private val content = Fixture.library.getValue(song.path)
  private val root: File by lazy { folder.newFolder("Tabs").also { File(it, song.path).apply { parentFile!!.mkdirs() }.writeBytes(content) } }

  // FileProvider keeps the folders it shares from in a static cache, but each test has its own app folders.
  @Before
  fun forgetFileProviderRoots() {
    (FileProvider::class.java.getDeclaredField("sCache").apply { isAccessible = true }.get(null) as MutableMap<*, *>).clear()
  }

  @Test
  fun aSongIsHandedOverAsACopyUnderItsRealName() {
    assertNull(runBlocking { TuxGuitar.open(activity, root.path, song) })
    val intent = shadowOf(activity).nextStartedActivity
    assertEquals(Intent.ACTION_VIEW, intent.action)
    assertEquals(TuxGuitar.PACKAGE, intent.`package`)
    assertTrue("grants read access", intent.flags and Intent.FLAG_GRANT_READ_URI_PERMISSION != 0)
    val uri = intent.data!!
    assertEquals("content", uri.scheme)
    assertEquals("${app.packageName}.files", uri.authority)
    assertEquals("Quartz.gp3", uri.lastPathSegment!!.substringAfterLast('/'))
    assertArrayEquals(content, app.contentResolver.openInputStream(uri)!!.use { it.readBytes() })
  }

  @Test
  fun aSongDeletedAfterTheScanIsAnError() {
    File(root, song.path).delete()
    val message = runBlocking { TuxGuitar.open(activity, root.path, song) }
    assertTrue(message, message!!.contains("Quartz.gpx.crdownload"))
    assertNull(shadowOf(activity).nextStartedActivity)
  }

  @Test
  fun withoutTuxGuitarTheAppSaysSo() {
    shadowOf(app).checkActivities(true) // starting an intent nothing handles throws, as on a device
    assertEquals("TuxGuitar is not installed", runBlocking { TuxGuitar.open(activity, root.path, song) })
  }
}
