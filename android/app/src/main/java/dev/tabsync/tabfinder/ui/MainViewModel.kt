package dev.tabsync.tabfinder.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.tabsync.tabfinder.data.Query
import dev.tabsync.tabfinder.data.SearchResult
import dev.tabsync.tabfinder.data.Song
import dev.tabsync.tabfinder.data.Sort
import dev.tabsync.tabfinder.data.TabSource
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.mapLatest
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class MainUiState(
  val root: String? = null, // absolute path of the tab folder
  val loaded: Boolean = false, // the saved scan has been read
  val songs: List<Song> = emptyList(),
  val scanning: Boolean = false,
  val message: String? = null,
)

/** The songs a search matched, with the suggestions for what's typed. */
data class Results(val songs: List<Song>, val search: SearchResult)

class MainViewModel(private val finder: TabSource) : ViewModel() {
  private val _state = MutableStateFlow(MainUiState(root = finder.root))
  val state: StateFlow<MainUiState> = _state

  val input = MutableStateFlow(Query())

  /** The songs by path, which a search's matches name; null until the saved scan has been read. */
  private val songsByPath: Flow<Map<String, Song>?> =
    _state
      .distinctUntilChanged { a, b -> a.loaded == b.loaded && a.songs === b.songs } // not on messages or the scanning flag
      .map { s -> if (s.loaded) s.songs.associateBy { it.path } else null }

  /**
   * Null until the saved scan has been read. A new query or new songs search anew, dropping a search
   * still running. Matches tabscan names that aren't among the songs (it answered for a newer scan
   * than they are) are left out; the search that follows the new songs fills them in.
   */
  @OptIn(ExperimentalCoroutinesApi::class)
  val results: StateFlow<Results?> =
    combine(songsByPath, input) { songs, q -> songs to q }
      .mapLatest { (songs, q) ->
        if (songs == null) return@mapLatest null
        val r = attempt { finder.search(q) }.getOrElse {
          showMessage("Search failed: ${it.message}")
          SearchResult()
        }
        Results(r.matches.mapNotNull { songs[it] }, r)
      }
      .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), null)

  init {
    viewModelScope.launch {
      val songs = attempt { finder.load() }.getOrElse {
        showMessage("Couldn't read the saved scan: ${it.message}")
        emptyList()
      }
      _state.update { it.copy(songs = songs, loaded = true) }
      if (songs.isEmpty() && finder.root != null) rescan()
    }
  }

  /** Drops the filters; the order stays. */
  fun clearFilters() {
    input.value = Query(sort = input.value.sort)
  }

  /** The range of levels a role's part must be in: 1 to 10 for any. */
  fun level(role: Role): IntRange = input.value.level(role)

  /** Sets the range of levels of a role's part; 1 to 10 is any, no filter. */
  fun setLevel(role: Role, range: IntRange) = input.update { it.withLevel(role, range) }

  fun clearLevel(role: Role) = setLevel(role, allLevels)

  fun setSort(sort: Sort) = input.update { it.copy(sort = sort) }

  fun setRoot(root: String) {
    _state.update { it.copy(root = root, songs = emptyList()) }
    viewModelScope.launch {
      finder.chooseRoot(root)
      rescan()
    }
  }

  fun rescan() {
    if (_state.value.scanning || _state.value.root == null) return
    _state.update { it.copy(scanning = true) }
    viewModelScope.launch {
      val result = attempt { finder.scan() }
      _state.update { s ->
        result.fold(
          { r -> s.copy(songs = r.songs, scanning = false, message = r.warning ?: r.summary) },
          { e -> s.copy(scanning = false, message = "Scan failed: ${e.message}") },
        )
      }
    }
  }

  fun showMessage(message: String) = _state.update { it.copy(message = message) }

  fun messageShown() = _state.update { it.copy(message = null) }
}

/**
 * [runCatching] for suspending calls: a cancellation isn't a failure but goes on, so a search that
 * mapLatest drops for a newer one isn't reported as failed.
 */
private inline fun <T> attempt(block: () -> T): Result<T> =
  try {
    Result.success(block())
  } catch (e: CancellationException) {
    throw e
  } catch (e: Throwable) {
    Result.failure(e)
  }
