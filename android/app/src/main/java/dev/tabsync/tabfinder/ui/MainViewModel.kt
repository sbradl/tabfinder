package dev.tabsync.tabfinder.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.tabsync.tabfinder.data.Query
import dev.tabsync.tabfinder.data.SearchResult
import dev.tabsync.tabfinder.data.Song
import dev.tabsync.tabfinder.data.TabSource
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
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

  /** Null until the saved scan has been read. Each change searches anew, dropping a search still running. */
  @OptIn(ExperimentalCoroutinesApi::class)
  val results: StateFlow<Results?> =
    combine(_state, input) { s, q -> s to q }
      .mapLatest { (s, q) ->
        if (!s.loaded) return@mapLatest null
        val r = runCatching { finder.search(q) }.getOrElse {
          showMessage("Search failed: ${it.message}")
          SearchResult()
        }
        Results(r.matches.mapNotNull { s.songs.getOrNull(it) }, r)
      }
      .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), null)

  init {
    viewModelScope.launch {
      val songs = runCatching { finder.load() }.getOrElse {
        showMessage("Couldn't read the saved scan: ${it.message}")
        emptyList()
      }
      _state.update { it.copy(songs = songs, loaded = true) }
      if (songs.isEmpty() && finder.root != null) rescan()
    }
  }

  fun clearFilters() {
    input.value = Query()
  }

  fun setRoot(root: String) {
    finder.root = root
    _state.update { it.copy(root = root, songs = emptyList()) }
    rescan()
  }

  fun rescan() {
    if (_state.value.scanning || _state.value.root == null) return
    _state.update { it.copy(scanning = true) }
    viewModelScope.launch {
      val result = runCatching { finder.scan() }
      _state.update { s ->
        result.fold(
          { r -> s.copy(songs = r.songs, scanning = false, message = r.warning ?: "${r.songs.size} tabs, ${r.unreadable} unreadable") },
          { e -> s.copy(scanning = false, message = "Scan failed: ${e.message}") },
        )
      }
    }
  }

  fun showMessage(message: String) = _state.update { it.copy(message = message) }

  fun messageShown() = _state.update { it.copy(message = null) }
}
