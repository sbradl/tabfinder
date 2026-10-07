package dev.tabsync.tabfinder.data

import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** U-AND-04 */
class QueryTest {
  @Test
  fun `an empty query is not active`() {
    assertFalse(Query().active)
  }

  @Test
  fun `each field alone makes it active`() {
    assertTrue(Query(name = "a").active)
    assertTrue(Query(artist = "a").active)
    assertTrue(Query(tuning = "a").active)
    assertTrue(Query(bpm = "1").active)
    assertTrue(Query(strings = 6).active)
  }

  @Test
  fun `blank text counts as typed`() {
    assertTrue(Query(name = " ").active) // as in the desktop app: the fields as typed
  }

  @Test
  fun `copy keeps the other fields`() {
    val q = Query(name = "n", artist = "a", tuning = "t", bpm = "1", strings = 7).copy(tuning = "", strings = 0)
    assertEquals(Query(name = "n", artist = "a", bpm = "1"), q)
  }
}

/** U-AND-05: Finder.treeToPath on the parts of a tree URI (Android's Uri itself isn't in JVM tests). */
class TreeToPathTest {
  private val external = "com.android.externalstorage.documents"
  private val primary = File("/storage/emulated/0")

  @Test
  fun `primary volume`() {
    assertEquals("/storage/emulated/0/Music/Tabs", Finder.treeToPath(external, "primary:Music/Tabs", primary))
    assertEquals("/storage/emulated/0/Tabs", Finder.treeToPath(external, "primary:Tabs", primary))
  }

  @Test
  fun `the root of the primary volume`() {
    assertEquals("/storage/emulated/0", Finder.treeToPath(external, "primary:", primary))
    assertEquals("/storage/emulated/0", Finder.treeToPath(external, "primary", primary))
  }

  @Test
  fun `an SD card`() {
    assertEquals("/storage/1234-ABCD/Tabs", Finder.treeToPath(external, "1234-ABCD:Tabs", primary))
    assertEquals("/storage/1234-ABCD", Finder.treeToPath(external, "1234-ABCD:", primary))
    assertEquals("/storage/1234-ABCD/a/b c/d", Finder.treeToPath(external, "1234-ABCD:a/b c/d", primary))
  }

  @Test
  fun `a colon in the path stays`() {
    assertEquals("/storage/emulated/0/a:b", Finder.treeToPath(external, "primary:a:b", primary))
  }

  @Test
  fun `other providers have no path`() {
    assertNull(Finder.treeToPath("com.android.providers.downloads.documents", "downloads", primary))
    assertNull(Finder.treeToPath("com.android.providers.downloads.documents", "primary:Download", primary))
    assertNull(Finder.treeToPath(null, "primary:Tabs", primary))
    assertNull(Finder.treeToPath("com.google.android.apps.docs.storage", "x:y", primary))
  }
}
