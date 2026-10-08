package dev.tabsync.tabfinder.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.InputChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RangeSlider
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.tabsync.tabfinder.R
import dev.tabsync.tabfinder.data.Part
import dev.tabsync.tabfinder.data.Query
import dev.tabsync.tabfinder.data.Sort
import dev.tabsync.tabfinder.theme.Mono
import kotlin.math.abs
import kotlin.math.roundToInt

// The difficulty of the parts, as in the desktop app (cmd/tabfinder/levels.go): a meter per part on the rows,
// a dialog with a range of levels per part, chips that sum the ranges up, and the order of the list.

/** A level's color, green for 1 to red for 10, dark enough to stand out against [background] (WCAG's 3:1 for graphics). */
fun levelColor(level: Int, background: Color): Color {
  val hue = 120f * (10 - level.coerceIn(1, 10)) / 9 // degrees
  var v = 0.85f
  var c = greenToRed(hue, v)
  while (contrast(c, background) < 3 && v > 0) {
    v -= 0.01f
    c = greenToRed(hue, v)
  }
  return c
}

/** The color of a hue between red (0°) and green (120°), at value [v]. */
private fun greenToRed(hue: Float, v: Float): Color {
  val s = 0.75f
  val c = v * s
  val x = c * (1 - abs((hue / 60) % 2 - 1))
  val m = v - c
  val (r, g) = if (hue < 60) c to x else x to c
  return Color(r + m, g + m, m)
}

/** WCAG's contrast ratio of two colors, 1 to 21. */
fun contrast(a: Color, b: Color): Double {
  val la = a.luminance() + 0.05
  val lb = b.luminance() + 0.05
  return (maxOf(la, lb) / minOf(la, lb)).toDouble()
}

/** The i'th of the meter's five bars, in dp: each taller than the one before. */
private fun meterBar(i: Int): Pair<Offset, Size> {
  val h = 6f + 3 * i
  return Offset(i * 6f, 18 - h) to Size(4f, h)
}

/** A level as five rising bars, two levels each; an odd level lights the lower half of its last bar. */
@Composable
fun Meter(level: Int, modifier: Modifier = Modifier) {
  val lit = levelColor(level, MaterialTheme.colorScheme.background)
  val dim = MaterialTheme.colorScheme.outline
  Canvas(modifier.size(28.dp, 18.dp)) {
    val dp = density
    for (i in 0 until 5) {
      val (at, size) = meterBar(i)
      drawRect(dim, at * dp, size * dp)
      when {
        level >= 2 * (i + 1) -> drawRect(lit, at * dp, size * dp)
        level == 2 * i + 1 -> drawRect(lit, Offset(at.x, at.y + size.height / 2) * dp, Size(size.width, size.height / 2) * dp)
      }
    }
  }
}

/** A role's instrument: a drum with sticks, a bass clef, a guitar, and a guitar with a spark for the lead. */
@Composable
fun RoleIcon(role: Role, size: Dp, color: Color, modifier: Modifier = Modifier) {
  Canvas(modifier.size(size)) { drawRole(role, color) }
}

private fun DrawScope.drawRole(role: Role, color: Color) {
  val f = this.size.width / 24 // drawn on a 24 grid
  fun pt(x: Float, y: Float) = Offset(x * f, y * f)
  fun stroke(w: Float, build: Path.() -> Unit) = drawPath(Path().apply(build), color, style = Stroke(w * f, cap = StrokeCap.Butt))
  fun fill(build: Path.() -> Unit) = drawPath(Path().apply(build).apply { close() }, color)
  fun dot(x: Float, y: Float, r: Float) = drawCircle(color, r * f, pt(x, y))
  fun Path.moveTo(p: Offset) = moveTo(p.x, p.y)
  fun Path.lineTo(p: Offset) = lineTo(p.x, p.y)
  fun Path.cubicTo(a: Offset, b: Offset, c: Offset) = cubicTo(a.x, a.y, b.x, b.y, c.x, c.y)
  fun guitar() {
    dot(8f, 17f, 5.5f)
    dot(12f, 12.5f, 4f)
    stroke(2.4f) { moveTo(pt(11f, 13f)); lineTo(pt(19f, 5f)) }
    fill { moveTo(pt(18f, 3.5f)); lineTo(pt(21f, 1f)); lineTo(pt(23f, 3f)); lineTo(pt(20.5f, 6f)) }
  }
  when (role) {
    Role.DRUMS -> {
      fill { moveTo(pt(3f, 12f)); lineTo(pt(21f, 12f)); lineTo(pt(21f, 19f)); cubicTo(pt(21f, 23f), pt(3f, 23f), pt(3f, 19f)) }
      stroke(1.6f) {
        moveTo(pt(3f, 11f))
        cubicTo(pt(3f, 7f), pt(21f, 7f), pt(21f, 11f))
        cubicTo(pt(21f, 14f), pt(3f, 14f), pt(3f, 11f))
      }
      stroke(1.8f) { moveTo(pt(4f, 2f)); lineTo(pt(11f, 9f)) }
      stroke(1.8f) { moveTo(pt(20f, 2f)); lineTo(pt(13f, 9f)) }
    }
    Role.BASS -> {
      dot(6f, 9f, 2.6f)
      stroke(2.4f) {
        moveTo(pt(4.5f, 8f))
        cubicTo(pt(5f, 2f), pt(17f, 2f), pt(17f, 9f))
        cubicTo(pt(17f, 15f), pt(11f, 19f), pt(4f, 21f))
      }
      dot(21f, 6.5f, 1.6f)
      dot(21f, 12.5f, 1.6f)
    }
    Role.RHYTHM -> guitar()
    Role.LEAD -> {
      guitar()
      stroke(1.6f) { moveTo(pt(2f, 5f)); lineTo(pt(8f, 5f)); moveTo(pt(5f, 2f)); lineTo(pt(5f, 8f)) }
    }
  }
}

/** A song's parts: per part its instrument, a meter and the level. */
@Composable
fun PartsLine(path: String, parts: List<Part>, modifier: Modifier = Modifier) {
  Row(modifier, horizontalArrangement = Arrangement.spacedBy(14.dp), verticalAlignment = Alignment.CenterVertically) {
    for (part in parts) {
      val role = Role.of(part.role) ?: continue
      Row(
        Modifier.testTag("part-$path-${role.key}").semantics(mergeDescendants = true) { contentDescription = "${role.label}, level ${part.level}" },
        horizontalArrangement = Arrangement.spacedBy(5.dp),
        verticalAlignment = Alignment.CenterVertically,
      ) {
        RoleIcon(role, 18.dp, MaterialTheme.colorScheme.onSurfaceVariant)
        Meter(part.level)
        Text("${part.level}", fontFamily = Mono, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
      }
    }
  }
}

/** The "Difficulty…" button and a chip per role with a range of levels, each with a ✕ that drops it. */
@Composable
fun LevelsLine(input: Query, onOpen: () -> Unit, onClear: (Role) -> Unit) {
  FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(4.dp), itemVerticalAlignment = Alignment.CenterVertically) {
    OutlinedButton(onClick = onOpen, modifier = Modifier.testTag("difficulty")) {
      Icon(painterResource(R.drawable.ic_tune), null, Modifier.size(18.dp), tint = MaterialTheme.colorScheme.primary)
      Text("Difficulty…", style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(start = 8.dp))
    }
    for (chip in input.chips()) {
      InputChip(
        selected = false,
        onClick = onOpen,
        label = { Text(chip.text, style = MaterialTheme.typography.labelLarge) },
        leadingIcon = { RoleIcon(chip.role, 18.dp, MaterialTheme.colorScheme.onSurface) },
        trailingIcon = {
          IconButton(onClick = { onClear(chip.role) }, Modifier.size(24.dp).testTag("chip-clear-${chip.role.key}")) {
            Icon(painterResource(R.drawable.ic_close), "Clear ${chip.role.label}", Modifier.size(16.dp))
          }
        },
        modifier = Modifier.testTag("chip-${chip.role.key}"),
      )
    }
  }
}

/** A range of levels per role; changes apply at once, so the list behind follows. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LevelsDialog(input: Query, onLevel: (Role, IntRange) -> Unit, onReset: () -> Unit, onDone: () -> Unit) {
  AlertDialog(
    onDismissRequest = onDone,
    modifier = Modifier.testTag("difficulty-dialog"),
    title = { Text("Difficulty") },
    text = {
      Column {
        Text("Levels 1 to 10, per part", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        for (role in Role.entries) {
          val range = input.level(role)
          Row(Modifier.padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            RoleIcon(role, 24.dp, MaterialTheme.colorScheme.onSurface)
            Column(Modifier.padding(start = 12.dp).weight(1f)) {
              Row {
                Text(role.label, style = MaterialTheme.typography.labelLarge, modifier = Modifier.weight(1f))
                Text(
                  if (range.first == range.last) "${range.first}" else "${range.first}–${range.last}",
                  fontFamily = Mono,
                  fontWeight = FontWeight.SemiBold,
                  color = if (range == allLevels) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.primary,
                )
              }
              Box(Modifier.testTag("slider-${role.key}")) {
                RangeSlider(
                  value = range.first.toFloat()..range.last.toFloat(),
                  onValueChange = { onLevel(role, it.start.roundToInt()..it.endInclusive.roundToInt()) },
                  valueRange = allLevels.first.toFloat()..allLevels.last.toFloat(),
                  steps = allLevels.last - allLevels.first - 1,
                )
              }
            }
          }
        }
      }
    },
    confirmButton = { Button(onClick = onDone, modifier = Modifier.testTag("difficulty-done")) { Text("Done") } },
    dismissButton = { TextButton(onClick = onReset, modifier = Modifier.testTag("difficulty-reset")) { Text("Reset") } },
  )
}

/** The order's button in the top bar, and its menu. */
@Composable
fun SortButton(sort: Sort, onSort: (Sort) -> Unit) {
  var open by remember { mutableStateOf(false) }
  Box {
    OutlinedButton(onClick = { open = true }, modifier = Modifier.testTag("sort-button")) {
      Icon(painterResource(R.drawable.ic_sort), "Sort", Modifier.size(18.dp), tint = MaterialTheme.colorScheme.primary)
      Text(sortLabel(sort), style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(start = 8.dp))
    }
    // Raised off the top bar like the suggestion menus: menus default to its own color.
    DropdownMenu(
      expanded = open,
      onDismissRequest = { open = false },
      containerColor = MaterialTheme.colorScheme.surfaceContainerHighest,
      border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
      shadowElevation = 12.dp,
      shape = RoundedCornerShape(10.dp),
    ) {
      for (s in Sort.entries) {
        DropdownMenuItem(
          text = { Text(sortName(s)) },
          onClick = {
            onSort(s)
            open = false
          },
          trailingIcon = if (s == sort) ({ Icon(painterResource(R.drawable.ic_check), "Current order", tint = MaterialTheme.colorScheme.primary) }) else null,
          modifier = Modifier.width(200.dp).testTag("sort-${sortKey(s)}"),
        )
      }
    }
  }
}

private fun sortKey(s: Sort) =
  when (s) {
    Sort.AZ -> "az"
    Sort.EASIEST -> "easiest"
    Sort.HARDEST -> "hardest"
  }

private fun sortName(s: Sort) =
  when (s) {
    Sort.AZ -> "A–Z"
    Sort.EASIEST -> "Easiest first"
    Sort.HARDEST -> "Hardest first"
  }

private fun sortLabel(s: Sort) =
  when (s) {
    Sort.AZ -> "A–Z"
    Sort.EASIEST -> "Easiest"
    Sort.HARDEST -> "Hardest"
  }
