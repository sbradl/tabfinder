package dev.tabsync.tabfinder.data

import dev.tabsync.tabfinder.Backend
import dev.tabsync.tabfinder.fakeBinary
import dev.tabsync.tabfinder.tg1
import dev.tabsync.tabfinder.tree
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

/** U-AND-02: the tabscan process dying, and U-AND-03: what its answers may look like. */
class FinderProcessTest {
  @get:Rule val tmp = TemporaryFolder()

  private fun scanned(): Pair<Backend, List<Song>> = runBlocking {
    val b = Backend(tmp)
    b.finder.root = tmp.tree("Soilbed Quartet/Brass Kettle.tg" to tg1("Brass Kettle", "Soilbed Quartet", ""), "Amber Marsh/First.tg" to tg1("First", "Amber Marsh", "")).path
    b to b.finder.scan().songs
  }

  /** The tabscan processes this JVM started. */
  private fun children() = ProcessHandle.current().children().filter { it.info().command().orElse("").endsWith("/tabscan") }.toList()

  @Test
  fun `a killed tabscan is restarted and the songs replayed`() = runBlocking {
    val (b, songs) = scanned()
    assertEquals(listOf(1), b.finder.search(Query(artist = "soil")).matches)
    repeat(3) { round ->
      val kids = children()
      assertEquals("round $round", 1, kids.size)
      kids.forEach { it.destroyForcibly() }
      kids.forEach { while (it.isAlive) Thread.sleep(5) }
      // The next search starts it again, loads the saved scan, and answers as before.
      val r = b.finder.search(Query(artist = "soil"))
      assertEquals("round $round", listOf(1), r.matches)
      assertEquals(songs.size, b.finder.search(Query()).matches.size)
    }
    b.finder.search(Query()) // leave nothing half-done
    children().forEach { it.destroyForcibly() }
  }

  @Test
  fun `a killed tabscan is restarted for a load or scan too`() = runBlocking {
    val (b, songs) = scanned()
    children().forEach { it.destroyForcibly().also { _ -> while (it.isAlive) Thread.sleep(5) } }
    assertEquals(songs, b.finder.load())
    children().forEach { it.destroyForcibly().also { _ -> while (it.isAlive) Thread.sleep(5) } }
    assertEquals(songs.size, b.finder.scan().songs.size)
    children().forEach { it.destroyForcibly() }
  }

  @Test
  fun `a tabscan that dies after reading a request is reported with its last log line`() {
    val fake = tmp.fakeBinary("read line; echo 'first line' >&2; echo 'boom: libfoo.so not found' >&2; exit 1")
    val b = Backend(tmp, fake)
    val e = runCatching { runBlocking { b.finder.search(Query()) } }.exceptionOrNull()
    assertTrue("$e", e is IllegalStateException)
    assertEquals("tabscan stopped: boom: libfoo.so not found", e!!.message)
  }

  @Test
  fun `a tabscan that exits at once is reported with its last log line`() {
    val fake = tmp.fakeBinary("echo 'boom: cannot execute' >&2; exit 1")
    val e = runCatching { runBlocking { Backend(tmp, fake).finder.load() } }.exceptionOrNull()
    assertTrue("$e", e != null)
    assertTrue("the error should name the last log line, was: $e", e!!.message!!.contains("boom: cannot execute"))
  }

  @Test
  fun `a tabscan that stops without a word`() {
    val fake = tmp.fakeBinary("read line; exit 0")
    val e = runCatching { runBlocking { Backend(tmp, fake).finder.load() } }.exceptionOrNull()
    assertEquals("tabscan stopped: no output", e?.message)
  }

  @Test
  fun `a missing binary is an error, not a hang`() {
    val e = runCatching { runBlocking { Backend(tmp, java.io.File("/no/such/tabscan")).finder.load() } }.exceptionOrNull()
    assertTrue("$e", e != null)
  }

  // --- U-AND-03 ---

  @Test
  fun `null lists in search answers read as empty and unknown fields are ignored`() = runBlocking {
    val fake = tmp.fakeBinary("""while read line; do echo '{"matches":null,"bpmInvalid":false,"artists":null,"tunings":null,"somethingNew":{"a":[1]}}'; done""")
    val r = Backend(tmp, fake).finder.search(Query())
    assertEquals(SearchResult(), r)
  }

  @Test
  fun `null lists in tunings read as empty`() = runBlocking {
    val fake =
      tmp.fakeBinary(
        """while read line; do echo '{"matches":[0],"artists":["A"],"tunings":[{"strings":6,"name":"Drop C","notes":"C G C F A D","label":"Drop C","detail":null,"extra":1}]}'; done"""
      )
    val r = Backend(tmp, fake).finder.search(Query())
    assertEquals(listOf(Tuning(6, "Drop C", "C G C F A D", "Drop C", "")), r.tunings)
  }

  @Test
  fun `a song with null lists reads as having none`() = runBlocking {
    val fake =
      tmp.fakeBinary(
        """while read line; do echo '{"songs":[{"path":"a/b.gp5","title":"T","artist":"A","album":"","tunings":null,"bpms":null,"unreadable":false,"openAs":"T.gp5","extra":true}],"unknown":1}'; done"""
      )
    val songs = Backend(tmp, fake).finder.load()
    assertEquals(1, songs.size)
    assertEquals(emptyList<Tuning>(), songs[0].tunings)
    assertEquals(emptyList<String>(), songs[0].bpms)
  }

  @Test
  fun `a null songs list reads as empty`() = runBlocking {
    val fake = tmp.fakeBinary("""while read line; do echo '{"songs":null}'; done""")
    assertEquals(emptyList<Song>(), Backend(tmp, fake).finder.load())
  }

  @Test
  fun `scan answers carry the warning and the unreadable count`() = runBlocking {
    val fake = tmp.fakeBinary("""while read line; do echo '{"songs":[],"unreadable":3,"warning":"/x: permission denied"}'; done""")
    val b = Backend(tmp, fake)
    b.finder.root = "/x"
    assertEquals(ScanResult(emptyList(), 3, "/x: permission denied"), b.finder.scan())
  }

  @Test
  fun `a garbled answer is an error`() {
    val fake = tmp.fakeBinary("""while read line; do echo 'this is not json'; done""")
    val e = runCatching { runBlocking { Backend(tmp, fake).finder.search(Query()) } }.exceptionOrNull()
    assertTrue("$e", e != null)
  }
}
