package dev.tabsync.tabfinder.data

import dev.tabsync.tabfinder.Backend
import dev.tabsync.tabfinder.hostTabscan
import dev.tabsync.tabfinder.isRoot
import dev.tabsync.tabfinder.lock
import dev.tabsync.tabfinder.tg1
import dev.tabsync.tabfinder.tree
import dev.tabsync.tabfinder.unlock
import java.io.File
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeFalse
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.Assert.fail

/** U-AND-01: Finder against the real (host) tabscan. */
class FinderTest {
  @get:Rule val tmp = TemporaryFolder()

  private fun library() =
    tmp.tree(
      "Soilbed Quartet/Glass Orchard/Brass Kettle.tg" to tg1("Brass Kettle", "Soilbed Quartet", "Glass Orchard"),
      "Amber Marsh/First Frost.tg" to tg1("First Frost", "Amber Marsh", "Tide of Lanterns"),
      "Inkwell Flamingos/Mirage.tg" to tg1("Mirage", "Inkwell Flamingos", "Mossman"),
      "INKWELL FLAMINGOS/Paper Ride.tg" to tg1("Paper Ride", "INKWELL FLAMINGOS", "Mossman"),
      "Broken/garbled.gp5" to "not a tab".toByteArray(),
      "notes.txt" to "hello".toByteArray(),
    )

  @Test
  fun `load without an index is empty`() = runBlocking {
    val b = Backend(tmp)
    assertEquals(emptyList<Song>(), b.finder.load())
  }

  @Test
  fun `scan lists the songs sorted, counts the unreadable ones and writes the index`() = runBlocking {
    val b = Backend(tmp)
    b.finder.chooseRoot(library().path)
    val r = b.finder.scan()
    assertEquals(listOf("Amber Marsh", "Broken", "Inkwell Flamingos", "INKWELL FLAMINGOS", "Soilbed Quartet"), r.songs.map { it.artist })
    assertEquals(listOf("First Frost", "Garbled", "Mirage", "Paper Ride", "Brass Kettle"), r.songs.map { it.title })
    assertEquals(1, r.songs.count { it.unreadable })
    assertNull(r.warning)
    assertTrue(r.songs.single { it.title == "Garbled" }.unreadable)
    assertEquals("Brass Kettle.tg", r.songs.last().openAs)
    // What the rows show comes worked out from tabscan (internal/rows).
    assertEquals("Soilbed Quartet · Glass Orchard", r.songs.last().subtitle)
    assertEquals("5 tabs, 1 unreadable", r.summary)
    assertTrue(b.index.isFile)
    // A second Finder over the same data folder reads the scan from the index.
    assertEquals(r.songs, Finder(hostTabscan(), b.dataDir, b.store).load())
  }

  @Test
  fun `scan of an empty folder`() = runBlocking {
    val b = Backend(tmp)
    b.finder.chooseRoot(tmp.newFolder("empty").path)
    val r = b.finder.scan()
    assertEquals(emptyList<Song>(), r.songs)
    assertEquals("0 tabs, 0 unreadable", r.summary)
    assertNull(r.warning)
  }

  @Test
  fun `an unreadable folder gives a warning and the songs before it`() = runBlocking {
    assumeFalse("root reads everything", isRoot())
    val root = library()
    val locked = File(root, "ZZZ locked").apply { mkdirs() }
    locked.lock()
    try {
      val b = Backend(tmp)
      b.finder.chooseRoot(root.path)
      val r = b.finder.scan()
      assertTrue(r.warning!!, r.warning!!.contains("ZZZ locked"))
      assertTrue(r.songs.isNotEmpty())
    } finally {
      locked.unlock()
    }
  }

  @Test
  fun `search round trip`() = runBlocking {
    val b = Backend(tmp)
    b.finder.chooseRoot(library().path)
    val songs = b.finder.scan().songs

    val all = b.finder.search(Query())
    assertEquals(songs.map { it.path }, all.matches)
    assertFalse(all.bpmInvalid)
    assertEquals(listOf("Amber Marsh", "Broken", "INKWELL FLAMINGOS", "Soilbed Quartet"), all.artists) // a tie of spellings goes to the first in byte order)

    val r = b.finder.search(Query(artist = "inkwell flamingos"))
    val byPath = songs.associateBy { it.path }
    assertEquals(listOf("Mirage", "Paper Ride"), r.matches.map { byPath.getValue(it).title })
    assertEquals(emptyList<String>(), b.finder.search(Query(artist = "nobody")).matches)
    assertTrue(b.finder.search(Query(bpm = "fast")).bpmInvalid)
    // Suggestions follow what's typed.
    assertEquals(listOf("Amber Marsh"), b.finder.search(Query(artist = "amber")).artists)
  }

  // A saved scan of songs with parts: what a scan of Guitar Pro files with notes writes.
  private fun ratedIndex(b: Backend) {
    fun song(title: String, vararg parts: String) =
      """{"path":"Copper Wolves/$title.gp5","format":"gp5","artist":"Copper Wolves","title":"$title","parts":[${parts.joinToString(",")}]}"""
    fun part(role: String, score: Double, vararg tags: String) =
      """{"role":"$role","tracks":[0],"score":$score,"tags":[${tags.joinToString(",") { "\"$it\"" }}]}"""
    b.index.writeText(
      listOf(
          """{"tabfinderIndex":3}""",
          song("Anvil", part("drums", 2.2), part("rhythm", 4.6, "syncopated")),
          song("Bellows", part("drums", 7.8, "double kick"), part("rhythm", 6.4, "fast"), part("lead", 9.1, "bends")),
          song("Cinder", part("rhythm", 7.0)),
          """{"path":"Copper Wolves/Dross.tg","format":"tg","artist":"Copper Wolves","title":"Dross"}""",
        )
        .joinToString("\n", postfix = "\n")
    )
  }

  @Test
  fun `songs come with their parts`() = runBlocking {
    val b = Backend(tmp)
    ratedIndex(b)
    val songs = b.finder.load().associateBy { it.title }
    assertEquals(listOf(Part("drums", 8, listOf("double kick")), Part("rhythm", 6, listOf("fast")), Part("lead", 9, listOf("bends"))), songs.getValue("Bellows").parts)
    assertEquals(emptyList<Part>(), songs.getValue("Dross").parts)
  }

  @Test
  fun `search by the levels of parts, in order of difficulty`() = runBlocking {
    val b = Backend(tmp)
    ratedIndex(b)
    val titles = b.finder.load().associate { it.path to it.title }
    suspend fun search(q: Query) = b.finder.search(q).matches.map { titles.getValue(it) }
    assertEquals(listOf("Anvil", "Bellows", "Cinder"), search(Query(rhythm = "1-10"))) // songs with a rhythm part
    assertEquals(listOf("Bellows", "Cinder"), search(Query(rhythm = "6-7")))
    assertEquals(listOf("Cinder", "Bellows"), search(Query(rhythm = "6-7", sort = Sort.HARDEST)))
    assertEquals(listOf("Anvil"), search(Query(drums = "-3")))
    assertEquals(listOf("Bellows"), search(Query(drums = "5-", lead = "9")))
    assertEquals(listOf("Anvil", "Cinder", "Bellows", "Dross"), search(Query(sort = Sort.EASIEST)))
  }

  @Test
  fun `search after load uses the loaded songs`() = runBlocking {
    val b = Backend(tmp)
    b.finder.chooseRoot(library().path)
    b.finder.scan()
    val fresh = Finder(hostTabscan(), b.dataDir, b.store)
    assertEquals(5, fresh.load().size)
    assertEquals(5, fresh.search(Query()).matches.size)
  }

  @Test
  fun `an error from tabscan becomes an exception with its message`() = runBlocking {
    val b = Backend(tmp)
    b.finder.chooseRoot(File(tmp.root, "does-not-exist").path)
    val e = runCatching { b.finder.scan() }.exceptionOrNull()
    assertTrue("$e", e is IllegalStateException)
    assertTrue(e!!.message!!, e.message!!.contains("does-not-exist"))
    // The session goes on after an error.
    b.finder.chooseRoot(library().path)
    assertEquals(5, b.finder.scan().songs.size)
  }

  @Test
  fun `scan without a folder`() = runBlocking {
    val b = Backend(tmp)
    val e = runCatching { b.finder.scan() }.exceptionOrNull()
    assertTrue("$e", e is IllegalStateException && e.message == "No tab folder chosen")
  }

  @Test
  fun `setting the folder deletes the saved scan and is remembered`() = runBlocking {
    val b = Backend(tmp)
    b.finder.chooseRoot(library().path)
    b.finder.scan()
    assertTrue(b.index.isFile)
    b.finder.chooseRoot("/somewhere/else")
    assertFalse(b.index.exists())
    assertEquals("/somewhere/else", b.store.root)
    assertEquals("/somewhere/else", b.finder.root)
    assertEquals(emptyList<Song>(), b.finder.load())
  }

  @Test
  fun `concurrent calls are answered in order`() = runBlocking {
    val b = Backend(tmp)
    b.finder.chooseRoot(library().path)
    b.finder.scan()
    val results = (1..50).map { i -> async(Dispatchers.Default) { b.finder.search(if (i % 2 == 0) Query(artist = "soil") else Query()) } }
    val answers = results.map { it.await() }
    answers.forEachIndexed { i, r ->
      val want = if ((i + 1) % 2 == 0) 1 else 5
      if (r.matches.size != want) fail("search ${i + 1}: ${r.matches.size} matches, want $want")
    }
  }
}
