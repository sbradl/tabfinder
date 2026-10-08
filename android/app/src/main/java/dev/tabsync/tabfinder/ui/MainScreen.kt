package dev.tabsync.tabfinder.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExposedDropdownMenuAnchorType
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.tabsync.tabfinder.R
import dev.tabsync.tabfinder.data.Query
import dev.tabsync.tabfinder.data.SearchResult
import dev.tabsync.tabfinder.data.Song
import dev.tabsync.tabfinder.data.Tuning
import dev.tabsync.tabfinder.theme.LocalStringColors
import dev.tabsync.tabfinder.theme.Mono
import kotlinx.coroutines.launch

/**
 * The app's one screen; the desktop app (cmd/tabfinder) looks the same. Platform glue comes in as parameters:
 * [onPickFolder] chooses the tab folder (calling [MainViewModel.setRoot]), [onOpen] opens a song in
 * TuxGuitar and returns why it couldn't, if it couldn't, and [hasAccess]/[onRequestAccess] gate on Android's
 * file permission.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainScreen(
  viewModel: MainViewModel,
  onPickFolder: () -> Unit,
  onOpen: suspend (root: String, song: Song) -> String?,
  hasAccess: Boolean = true,
  onRequestAccess: () -> Unit = {},
) {
  val state by viewModel.state.collectAsStateWithLifecycle()
  val input by viewModel.input.collectAsStateWithLifecycle()
  val results by viewModel.results.collectAsStateWithLifecycle()
  val snackbar = remember { SnackbarHostState() }
  val scope = rememberCoroutineScope()
  var levelsOpen by remember { mutableStateOf(false) }

  LaunchedEffect(state.message) {
    state.message?.let {
      snackbar.showSnackbar(it)
      viewModel.messageShown()
    }
  }

  Scaffold(
    topBar = {
      TopAppBar(
        title = {
          Column {
            Text("TabFinder", style = MaterialTheme.typography.titleLarge)
            state.root?.let {
              Text(
                "${state.songs.size} tabs in ${it.trimEnd('/').substringAfterLast('/')}".uppercase(),
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
              )
            }
          }
        },
        actions = {
          if (hasAccess && state.root != null) SortButton(input.sort, viewModel::setSort)
          if (hasAccess) {
            IconButton(onClick = onPickFolder) { Icon(painterResource(R.drawable.ic_folder), "Choose tab folder") }
            IconButton(onClick = viewModel::rescan, enabled = state.root != null && !state.scanning) {
              Icon(painterResource(R.drawable.ic_refresh), "Rescan")
            }
          }
        },
        colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
      )
    },
    snackbarHost = { SnackbarHost(snackbar) },
  ) { padding ->
    val modifier = Modifier.padding(padding).fillMaxSize()
    when {
      !hasAccess ->
        Prompt("Allow file access", "TabFinder reads your tab folder directly to find artists, tunings and tempos.", "Allow access", modifier, onRequestAccess)
      state.root == null ->
        Prompt("Choose your tab folder", "Pick the folder that holds your Guitar Pro, TuxGuitar and Power Tab files.", "Choose folder", modifier, onPickFolder)
      else ->
        Column(modifier) {
          Filters(
            input,
            results?.search ?: SearchResult(),
            matching = results?.songs?.size ?: 0,
            total = state.songs.size,
            onChange = { viewModel.input.value = it },
            onLevels = { levelsOpen = true },
            onClearLevel = viewModel::clearLevel,
          )
          if (levelsOpen) {
            LevelsDialog(
              input,
              onLevel = viewModel::setLevel,
              onReset = { Role.entries.forEach(viewModel::clearLevel) },
              onDone = { levelsOpen = false },
            )
          }
          // Rows draw their own top divider, so the first one closes off the filter panel.
          if (state.scanning) LinearProgressIndicator(Modifier.fillMaxWidth())
          when (listContent(results, state, input)) {
            ListContent.LOADING -> {} // Reading the cached index; takes a moment on a large library.
            ListContent.NO_TABS ->
              Prompt("No tabs found", "Rescan, or choose another folder.", "Rescan", Modifier.fillMaxSize(), viewModel::rescan)
            ListContent.NO_MATCHES ->
              Prompt("No tabs match", "Try fewer filters.", "Clear filters", Modifier.fillMaxSize(), viewModel::clearFilters)
            ListContent.SONGS -> SongList(
              results!!.songs,
              onOpen = { song ->
                val root = state.root!!
                scope.launch { onOpen(root, song)?.let(viewModel::showMessage) }
              },
              query = results!!.query,
            )
          }
        }
    }
  }
}

/** What the list below the filters shows. */
internal enum class ListContent { LOADING, NO_TABS, NO_MATCHES, SONGS }

/**
 * "No tabs found" goes by the library, not by the last search: right after a scan, the results are
 * those for the songs before it until the search for the new ones answers.
 */
internal fun listContent(results: Results?, state: MainUiState, input: Query): ListContent =
  when {
    results == null -> ListContent.LOADING
    state.scanning -> ListContent.SONGS
    state.songs.isEmpty() -> ListContent.NO_TABS
    results.songs.isEmpty() && input.active -> ListContent.NO_MATCHES
    else -> ListContent.SONGS
  }

@Composable
private fun Prompt(title: String, text: String, action: String, modifier: Modifier = Modifier, onClick: () -> Unit) {
  Box(modifier.padding(32.dp).testTag("prompt"), contentAlignment = Alignment.Center) {
    Column(Modifier.widthIn(max = 420.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
      Text(title, style = MaterialTheme.typography.titleLarge)
      Text(text, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
      Button(onClick = onClick, modifier = Modifier.padding(top = 8.dp).testTag("prompt-action")) { Text(action, style = MaterialTheme.typography.labelLarge) }
    }
  }
}

@Composable
private fun Filters(
  input: Query,
  suggestions: SearchResult,
  matching: Int,
  total: Int,
  onChange: (Query) -> Unit,
  onLevels: () -> Unit,
  onClearLevel: (Role) -> Unit,
) {
  Surface(color = MaterialTheme.colorScheme.surfaceContainer) {
    BoxWithConstraints(Modifier.padding(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 8.dp)) {
      val wide = maxWidth >= 600.dp
      Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
          SuggestField(
            input.artist,
            { onChange(input.copy(artist = it)) },
            "Artist",
            suggestions.artists,
            { it },
            { onChange(input.copy(artist = it)) },
            Modifier.weight(1f),
          )
          FilterField(
            input.name,
            { onChange(input.copy(name = it)) },
            "Song",
            Modifier.weight(1.4f),
            leading = { Icon(painterResource(R.drawable.ic_search), null) },
          )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
          SuggestField(
            input.tuning,
            // Typing (or clearing) drops the string count a picked suggestion set.
            { onChange(input.copy(tuning = it, strings = 0)) },
            "Tuning",
            suggestions.tunings,
            text = { it.label },
            onPick = { onChange(input.copy(tuning = it.label, strings = it.strings)) },
            modifier = Modifier.weight(1f),
            detail = { it.detail },
            section = { "${it.strings} STRINGS" },
            leading = input.strings.takeIf { it != 0 }?.let { n -> { StringBadge(n, Modifier.padding(start = 12.dp)) } },
          )
          FilterField(
            input.bpm,
            { onChange(input.copy(bpm = it)) },
            "BPM",
            Modifier.width(if (wide) 150.dp else 112.dp),
            placeholder = "100-140",
            isError = suggestions.bpmInvalid,
            keyboardType = KeyboardType.Number,
          )
          Text(
            // Padded to the total's width so the monospace counter, and the fields beside it, never shift.
            "${matching.toString().padStart(total.toString().length)} / $total",
            fontFamily = Mono,
            fontWeight = FontWeight.SemiBold,
            fontSize = 16.sp,
            color = MaterialTheme.colorScheme.primary,
            maxLines = 1,
            modifier = Modifier.padding(horizontal = 8.dp).testTag("counter"),
          )
        }
        LevelsLine(input, onLevels, onClearLevel)
      }
    }
  }
}

@Composable
private fun FilterField(
  value: String,
  onValueChange: (String) -> Unit,
  label: String,
  modifier: Modifier = Modifier,
  placeholder: String? = null,
  isError: Boolean = false,
  keyboardType: KeyboardType = KeyboardType.Text,
  leading: (@Composable () -> Unit)? = null,
  trailing: (@Composable () -> Unit)? = null,
) {
  OutlinedTextField(
    value,
    onValueChange,
    label = { Text(label) },
    placeholder = placeholder?.let { { Text(it) } },
    leadingIcon = leading,
    trailingIcon = {
      if (value.isNotEmpty()) {
        IconButton(onClick = { onValueChange("") }, Modifier.testTag("clear-${label.lowercase()}")) { Icon(painterResource(R.drawable.ic_close), "Clear $label") }
      } else {
        trailing?.invoke()
      }
    },
    isError = isError,
    singleLine = true,
    keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
    shape = RoundedCornerShape(10.dp),
    modifier = modifier.testTag("field-${label.lowercase()}"),
  )
}

/**
 * Free text with suggestions, already matched to what's typed (by tabscan, see Finder). With
 * [section], they get a header wherever the section changes.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun <T> SuggestField(
  value: String,
  onValueChange: (String) -> Unit,
  label: String,
  options: List<T>,
  text: (T) -> String,
  onPick: (T) -> Unit,
  modifier: Modifier = Modifier,
  detail: ((T) -> String)? = null,
  section: ((T) -> String)? = null,
  leading: (@Composable () -> Unit)? = null,
) {
  var expanded by remember { mutableStateOf(false) }
  val show = expanded && options.isNotEmpty()
  ExposedDropdownMenuBox(expanded = show, onExpandedChange = { expanded = it }, modifier = modifier) {
    FilterField(
      value,
      {
        onValueChange(it)
        expanded = it.isNotEmpty()
      },
      label,
      Modifier.menuAnchor(ExposedDropdownMenuAnchorType.PrimaryEditable).fillMaxWidth(),
      leading = leading,
    )
    // Menus default to surfaceContainer, the filter panel's own color; raise it off the panel.
    ExposedDropdownMenu(
      expanded = show,
      onDismissRequest = { expanded = false },
      containerColor = MaterialTheme.colorScheme.surfaceContainerHighest,
      border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
      shadowElevation = 12.dp,
      shape = RoundedCornerShape(10.dp),
    ) {
      var lastSection: String? = null
      options.forEach { item ->
        section?.invoke(item)?.takeIf { it != lastSection }?.let { header ->
          if (lastSection != null) HorizontalDivider(Modifier.padding(vertical = 4.dp), color = MaterialTheme.colorScheme.outline)
          Text(
            header,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.primary,
            modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 4.dp).testTag("section-$header"),
          )
          lastSection = header
        }
        DropdownMenuItem(
          modifier = Modifier.testTag("suggestion-${label.lowercase()}-${text(item)}"),
          text = { Text(text(item), maxLines = 1, overflow = TextOverflow.Ellipsis) },
          trailingIcon =
            detail?.invoke(item)?.takeIf { it.isNotEmpty() }?.let { d ->
              { Text(d, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            },
          onClick = {
            onPick(item)
            expanded = false
          },
        )
      }
    }
  }
}

/**
 * The songs, from the top again for each [query] they answer: a list that kept its place would follow
 * the song on top to wherever a new order or filter puts it. Songs of a rescan keep the place.
 */
@Composable
internal fun SongList(songs: List<Song>, onOpen: (Song) -> Unit, modifier: Modifier = Modifier, query: Query = Query()) {
  val state = remember(query) { LazyListState() }
  LazyColumn(modifier.testTag("songs"), state = state) {
    items(songs, key = { it.path }) { song -> SongRow(song, Modifier.testTag("song-${song.path}").clickable { onOpen(song) }) }
  }
}

@Composable
private fun SongRow(song: Song, modifier: Modifier = Modifier) {
  Column(modifier.fillMaxWidth()) {
    HorizontalDivider(color = MaterialTheme.colorScheme.outline)
    Column(Modifier.padding(horizontal = 16.dp, vertical = 18.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
      Row(verticalAlignment = Alignment.Top) {
        Column(Modifier.weight(1f)) {
          Text(song.title, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
          Text(
            song.subtitle,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
          )
        }
        TempoText(song, Modifier.padding(start = 12.dp))
      }
      if (song.tunings.isNotEmpty() || song.parts.isNotEmpty()) {
        // The parts at the right of the tuning tags, or on a line of their own when both don't fit.
        FlowRow(
          horizontalArrangement = Arrangement.spacedBy(6.dp),
          verticalArrangement = Arrangement.spacedBy(6.dp),
          itemVerticalAlignment = Alignment.CenterVertically,
        ) {
          song.tunings.forEach { TuningTag(it) }
          if (song.parts.isNotEmpty()) {
            Box(Modifier.weight(1f), contentAlignment = Alignment.CenterEnd) { PartsLine(song.path, song.parts) }
          }
        }
      } else if (song.unreadable) {
        Text("Couldn't read this file", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
      }
    }
  }
}

/** The opening tempo, large, with any later tempo changes beneath. */
@Composable
private fun TempoText(song: Song, modifier: Modifier = Modifier) {
  if (song.tempo.isEmpty()) return
  Column(modifier, horizontalAlignment = Alignment.End) {
    Text(song.tempo, fontFamily = Mono, fontWeight = FontWeight.SemiBold, fontSize = 20.sp, lineHeight = 24.sp)
    Text(
      song.tempoDetail,
      style = MaterialTheme.typography.labelSmall,
      color = MaterialTheme.colorScheme.onSurfaceVariant,
    )
  }
}

/** The string count, colored by range (bass, guitar, extended). */
@Composable
private fun StringBadge(strings: Int, modifier: Modifier = Modifier) {
  val (container, content) = LocalStringColors.current.forStrings(strings)
  Box(modifier.clip(RoundedCornerShape(6.dp)).background(container).size(26.dp), contentAlignment = Alignment.Center) {
    Text("$strings", fontFamily = Mono, fontWeight = FontWeight.Bold, fontSize = 13.sp, color = content)
  }
}

/** "[7] Drop A" */
@Composable
private fun TuningTag(tuning: Tuning) {
  Surface(shape = RoundedCornerShape(6.dp), border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant), color = MaterialTheme.colorScheme.surface) {
    Row(verticalAlignment = Alignment.CenterVertically) {
      StringBadge(tuning.strings)
      Text(tuning.label, style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(horizontal = 8.dp))
    }
  }
}
