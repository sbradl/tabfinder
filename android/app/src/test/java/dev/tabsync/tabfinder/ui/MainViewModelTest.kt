package dev.tabsync.tabfinder.ui

import dev.tabsync.tabfinder.data.Query
import dev.tabsync.tabfinder.data.ScanResult
import dev.tabsync.tabfinder.data.SearchResult
import dev.tabsync.tabfinder.data.Sort
import dev.tabsync.tabfinder.data.Song
import dev.tabsync.tabfinder.data.TabSource
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

fun song(title: String, artist: String = "A") = Song("$artist/$title.gp5", title, artist, "", emptyList(), false, "$title.gp5")

/** A TabSource whose answers and delays the test controls. */
class FakeSource(override var root: String? = null) : TabSource {
  override suspend fun chooseRoot(path: String) {
    root = path
  }

  var saved: List<Song> = emptyList()
  var loadFails: Throwable? = null
  var loadGate: CompletableDeferred<Unit>? = null
  var scanResult: Result<ScanResult> = Result.success(ScanResult())
  var scanGate: CompletableDeferred<Unit>? = null
  var searchDelays: (Query) -> Long = { 0 }
  var searchFails: (Query) -> Throwable? = { null }
  var searchAnswer: (Query) -> SearchResult = { SearchResult(matches = saved.map { it.path }) }

  val loads = mutableListOf<Unit>()
  val scans = mutableListOf<String?>()
  val searches = mutableListOf<Query>()
  val finished = mutableListOf<Query>()

  override suspend fun load(): List<Song> {
    loads += Unit
    loadGate?.await()
    loadFails?.let { throw it }
    return saved
  }

  override suspend fun scan(): ScanResult {
    scans += root
    scanGate?.await()
    return scanResult.getOrThrow().also { saved = it.songs }
  }

  override suspend fun search(query: Query): SearchResult {
    searches += query
    delay(searchDelays(query))
    searchFails(query)?.let { throw it }
    finished += query
    return searchAnswer(query)
  }
}

/** U-AND-06 */
@OptIn(ExperimentalCoroutinesApi::class)
class MainViewModelTest {
  private val dispatcher = StandardTestDispatcher()

  @Before fun setUp() = Dispatchers.setMain(dispatcher)

  @After fun tearDown() = Dispatchers.resetMain()

  private fun TestScope.vm(source: FakeSource): MainViewModel {
    val vm = MainViewModel(source)
    backgroundScope.launch(dispatcher) { vm.results.collect {} } // results only run while someone listens
    return vm
  }

  @Test
  fun `results are null until the saved scan is loaded`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply {
      saved = listOf(song("One"), song("Two"))
      loadGate = CompletableDeferred()
    }
    val vm = vm(source)
    advanceUntilIdle()
    assertNull(vm.results.value)
    assertEquals(false, vm.state.value.loaded)
    source.loadGate!!.complete(Unit)
    advanceUntilIdle()
    assertEquals(true, vm.state.value.loaded)
    assertEquals(listOf("One", "Two"), vm.results.value!!.songs.map { it.title })
    assertEquals(0, source.scans.size) // there was a saved scan
  }

  @Test
  fun `the initial state has the folder`() = runTest(dispatcher) {
    val vm = vm(FakeSource("/tabs/Guitar"))
    assertEquals("/tabs/Guitar", vm.state.value.root)
    assertNull(vm.state.value.message)
  }

  @Test
  fun `an empty saved scan with a folder rescans by itself`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { scanResult = Result.success(ScanResult(listOf(song("New")))) }
    val vm = vm(source)
    advanceUntilIdle()
    assertEquals(listOf<String?>("/tabs"), source.scans)
    assertEquals(listOf("New"), vm.state.value.songs.map { it.title })
    assertEquals(false, vm.state.value.scanning)
  }

  @Test
  fun `without a folder or a saved scan there is nothing to scan`() = runTest(dispatcher) {
    val source = FakeSource(null)
    val vm = vm(source)
    advanceUntilIdle()
    assertTrue(source.scans.isEmpty())
    assertEquals(true, vm.state.value.loaded)
    vm.rescan()
    advanceUntilIdle()
    assertTrue(source.scans.isEmpty())
    assertEquals(false, vm.state.value.scanning)
  }

  @Test
  fun `a saved scan that can't be read shows a message and scans again`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply {
      loadFails = IllegalStateException("bad index")
      scanGate = CompletableDeferred()
    }
    val vm = vm(source)
    advanceUntilIdle()
    assertEquals("Couldn't read the saved scan: bad index", vm.state.value.message)
    assertEquals(true, vm.state.value.loaded)
    assertEquals(true, vm.state.value.scanning)
    source.scanGate!!.complete(Unit)
    advanceUntilIdle()
  }

  @Test
  fun `rescan is ignored while scanning`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply {
      saved = listOf(song("One"))
      scanGate = CompletableDeferred()
    }
    val vm = vm(source)
    advanceUntilIdle()
    vm.rescan()
    assertEquals(true, vm.state.value.scanning)
    vm.rescan()
    vm.rescan()
    advanceUntilIdle()
    assertEquals(1, source.scans.size)
    source.scanGate!!.complete(Unit)
    advanceUntilIdle()
    assertEquals(false, vm.state.value.scanning)
    vm.rescan() // and works again afterwards
    advanceUntilIdle()
    assertEquals(2, source.scans.size)
  }

  @Test
  fun `setRoot clears the songs and rescans`() = runTest(dispatcher) {
    val source = FakeSource("/old").apply {
      saved = listOf(song("Old"))
      scanResult = Result.success(ScanResult(listOf(song("Fresh"), song("Fresher"))))
    }
    val vm = vm(source)
    advanceUntilIdle()
    assertEquals(listOf("Old"), vm.state.value.songs.map { it.title })
    source.scanGate = CompletableDeferred()
    vm.setRoot("/new")
    assertEquals("/new", vm.state.value.root)
    assertTrue(vm.state.value.songs.isEmpty())
    advanceUntilIdle() // the folder is chosen off the main thread, then the scan starts
    assertEquals("/new", source.root)
    assertEquals(true, vm.state.value.scanning)
    assertEquals(listOf<String?>("/new"), source.scans)
    source.scanGate!!.complete(Unit)
    advanceUntilIdle()
    assertEquals(listOf("Fresh", "Fresher"), vm.state.value.songs.map { it.title })
    assertEquals(listOf("Fresh", "Fresher"), vm.results.value!!.songs.map { it.title })
  }

  @Test
  fun `a scan reports its tabs and unreadable files`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { saved = listOf(song("x")) }
    val vm = vm(source)
    advanceUntilIdle()
    source.scanResult = Result.success(ScanResult(listOf(song("a"), song("b"), song("c")), summary = "3 tabs, 2 unreadable"))
    vm.rescan()
    advanceUntilIdle()
    assertEquals("3 tabs, 2 unreadable", vm.state.value.message)
    assertEquals(3, vm.state.value.songs.size)
  }

  @Test
  fun `a scan warning is shown instead of the counts`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { saved = listOf(song("x")) }
    val vm = vm(source)
    advanceUntilIdle()
    source.scanResult = Result.success(ScanResult(listOf(song("a")), warning = "/tabs/locked: permission denied"))
    vm.rescan()
    advanceUntilIdle()
    assertEquals("/tabs/locked: permission denied", vm.state.value.message)
    assertEquals(1, vm.state.value.songs.size)
  }

  @Test
  fun `a failed scan says so and keeps the songs`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { saved = listOf(song("keep")) }
    val vm = vm(source)
    advanceUntilIdle()
    source.scanResult = Result.failure(IllegalStateException("no such folder"))
    vm.rescan()
    advanceUntilIdle()
    assertEquals("Scan failed: no such folder", vm.state.value.message)
    assertEquals(false, vm.state.value.scanning)
    assertEquals(listOf("keep"), vm.state.value.songs.map { it.title })
  }

  @Test
  fun `a failed search shows a message and no results`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply {
      saved = listOf(song("One"))
      searchFails = { if (it.artist == "boom") IllegalStateException("tabscan stopped") else null }
    }
    val vm = vm(source)
    advanceUntilIdle()
    assertEquals(1, vm.results.value!!.songs.size)
    vm.input.value = Query(artist = "boom")
    advanceUntilIdle()
    assertEquals("Search failed: tabscan stopped", vm.state.value.message)
    assertEquals(emptyList<Song>(), vm.results.value!!.songs)
    assertEquals(SearchResult(), vm.results.value!!.search)
    // The next query works again.
    vm.input.value = Query()
    advanceUntilIdle()
    assertEquals(1, vm.results.value!!.songs.size)
  }

  @Test
  fun `only the last of quick input changes is applied`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply {
      saved = listOf(song("One"), song("Two"), song("Three"))
      // The earlier the query, the slower the answer: they'd arrive in reverse order.
      searchDelays = { 100L * (5 - it.name.length) }
      searchAnswer = { q -> SearchResult(matches = listOf(saved[q.name.length - 1].path)) }
    }
    val vm = vm(source)
    advanceUntilIdle()
    source.searches.clear()
    source.finished.clear()
    vm.input.value = Query(name = "a")
    advanceTimeBy(10)
    vm.input.value = Query(name = "ab")
    advanceTimeBy(10)
    vm.input.value = Query(name = "abc")
    advanceUntilIdle()
    assertEquals(listOf("a", "ab", "abc"), source.searches.map { it.name }.distinct())
    assertEquals("the older searches were dropped before they finished", listOf("abc"), source.finished.map { it.name })
    assertEquals(listOf("Three"), vm.results.value!!.songs.map { it.title })
  }

  @Test
  fun `a search dropped for new songs isn't reported as failed`() = runTest(dispatcher) {
    // A first start: nothing saved, so the folder is scanned, and the search that runs meanwhile
    // is dropped when the scan's songs arrive.
    val source = FakeSource("/tabs").apply {
      scanGate = CompletableDeferred()
      scanResult = Result.success(ScanResult(listOf(song("One")), summary = "1 tab"))
      searchDelays = { 100 }
    }
    val vm = vm(source)
    advanceTimeBy(10)
    source.scanGate!!.complete(Unit)
    advanceUntilIdle()
    assertEquals("1 tab", vm.state.value.message)
    assertEquals(listOf("One"), vm.results.value!!.songs.map { it.title })
  }

  @Test
  fun `the songs a search matched are looked up by path, ignoring unknown ones`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply {
      saved = listOf(song("One"), song("Two"))
      searchAnswer = { SearchResult(matches = listOf("A/Two.gp5", "A/Seven.gp5", "A/One.gp5")) } // Seven is from a newer scan
    }
    val vm = vm(source)
    advanceUntilIdle()
    assertEquals(listOf("Two", "One"), vm.results.value!!.songs.map { it.title })
  }

  @Test
  fun `messages and the scanning flag don't search again`() = runTest(dispatcher) {
    val gate = CompletableDeferred<Unit>()
    val source = FakeSource("/tabs").apply {
      saved = listOf(song("One"))
      scanGate = gate
      scanResult = Result.success(ScanResult(saved))
    }
    val vm = vm(source)
    advanceUntilIdle()
    val before = source.searches.size
    vm.showMessage("Hello")
    advanceUntilIdle()
    vm.messageShown()
    advanceUntilIdle()
    vm.rescan()
    advanceUntilIdle()
    assertTrue(vm.state.value.scanning)
    assertEquals("no search for a message or while scanning", before, source.searches.size)
    gate.complete(Unit)
    advanceUntilIdle()
  }

  @Test
  fun `clearFilters resets the query`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { saved = listOf(song("One")) }
    val vm = vm(source)
    advanceUntilIdle()
    vm.input.value = Query(name = "n", artist = "a", tuning = "t", bpm = "1", strings = 6)
    advanceUntilIdle()
    assertTrue(vm.input.value.active)
    vm.clearFilters()
    advanceUntilIdle()
    assertEquals(Query(), vm.input.value)
    assertEquals(Query(), source.searches.last())
  }

  @Test
  fun `levels per part go into the query and come back as ranges`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { saved = listOf(song("One")) }
    val vm = vm(source)
    advanceUntilIdle()
    assertEquals(1..10, vm.level(Role.RHYTHM))
    vm.setLevel(Role.RHYTHM, 5..7)
    vm.setLevel(Role.DRUMS, 1..3)
    advanceUntilIdle()
    assertEquals(Query(rhythm = "5-7", drums = "1-3"), source.searches.last())
    assertEquals(5..7, vm.level(Role.RHYTHM))
    // The whole range is no filter: songs without the part count too.
    vm.setLevel(Role.DRUMS, 1..10)
    assertEquals("", vm.input.value.drums)
    vm.clearLevel(Role.RHYTHM)
    assertEquals(Query(), vm.input.value)
    // Open ends, as tabscan takes them, are the ends of the range.
    vm.input.value = Query(bass = "-3", lead = "7-")
    assertEquals(1..3, vm.level(Role.BASS))
    assertEquals(7..10, vm.level(Role.LEAD))
  }

  @Test
  fun `chips sum up the ranges, in the order of the parts`() {
    assertEquals(emptyList<LevelChip>(), Query().chips())
    assertEquals(
      listOf(LevelChip(Role.DRUMS, "7–10"), LevelChip(Role.RHYTHM, "5–7"), LevelChip(Role.LEAD, "8")),
      Query(lead = "8-8", rhythm = "5-7", drums = "7-").chips(),
    )
  }

  @Test
  fun `the order goes into the query, and clearing the filters keeps it`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { saved = listOf(song("One")) }
    val vm = vm(source)
    advanceUntilIdle()
    vm.setSort(Sort.HARDEST)
    advanceUntilIdle()
    assertEquals(Query(sort = Sort.HARDEST), source.searches.last())
    vm.setLevel(Role.BASS, 2..4)
    vm.input.value = vm.input.value.copy(artist = "a")
    vm.clearFilters()
    advanceUntilIdle()
    assertEquals(Query(sort = Sort.HARDEST), vm.input.value)
  }

  @Test
  fun `showMessage and messageShown`() = runTest(dispatcher) {
    val vm = vm(FakeSource(null))
    advanceUntilIdle()
    vm.showMessage("Hello")
    assertEquals("Hello", vm.state.value.message)
    vm.messageShown()
    assertNull(vm.state.value.message)
    vm.messageShown() // nothing to clear is fine
    assertNull(vm.state.value.message)
  }

  @Test
  fun `a search runs again when the songs change`() = runTest(dispatcher) {
    val source = FakeSource("/tabs").apply { saved = listOf(song("One")) }
    val vm = vm(source)
    advanceUntilIdle()
    val before = source.searches.size
    source.scanResult = Result.success(ScanResult(listOf(song("a"), song("b"))))
    vm.rescan()
    advanceUntilIdle()
    assertTrue("a search after the rescan", source.searches.size > before)
    assertEquals(2, vm.results.value!!.songs.size)
    assertNotNull(vm.results.value)
  }
}
