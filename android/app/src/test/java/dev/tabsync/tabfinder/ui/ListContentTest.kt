package dev.tabsync.tabfinder.ui

import dev.tabsync.tabfinder.data.Query
import dev.tabsync.tabfinder.data.SearchResult
import org.junit.Assert.assertEquals
import org.junit.Test

class ListContentTest {
  private val library = MainUiState(root = "/tabs", loaded = true, songs = listOf(song("One"), song("Two")))
  private fun results(vararg titles: String) = Results(titles.map { song(it) }, SearchResult())

  @Test
  fun `nothing while the saved scan is read`() {
    assertEquals(ListContent.LOADING, listContent(null, library, Query()))
  }

  @Test
  fun `no tabs only when the library is empty`() {
    assertEquals(ListContent.NO_TABS, listContent(results(), library.copy(songs = emptyList()), Query()))
    assertEquals(ListContent.NO_TABS, listContent(results(), library.copy(songs = emptyList()), Query(name = "x")))
  }

  @Test
  fun `a scan that found tabs isn't "no tabs" while its search is still running`() {
    // The results are still the empty ones from before the scan.
    assertEquals(ListContent.SONGS, listContent(results(), library, Query()))
  }

  @Test
  fun `no matches when filters leave nothing`() {
    assertEquals(ListContent.NO_MATCHES, listContent(results(), library, Query(name = "x")))
  }

  @Test
  fun `songs while scanning, and when something matches`() {
    assertEquals(ListContent.SONGS, listContent(results(), library.copy(songs = emptyList(), scanning = true), Query()))
    assertEquals(ListContent.SONGS, listContent(results("One"), library, Query(name = "o")))
  }
}
