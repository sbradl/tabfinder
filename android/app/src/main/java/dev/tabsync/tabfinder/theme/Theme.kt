package dev.tabsync.tabfinder.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

private val DarkColorScheme =
  darkColorScheme(
    primary = Brass,
    onPrimary = Graphite950,
    primaryContainer = BrassTintDark,
    onPrimaryContainer = Brass,
    secondaryContainer = Graphite800,
    onSecondaryContainer = Bone,
    tertiary = BassBlue,
    background = Graphite950,
    onBackground = Bone,
    surface = Graphite950,
    onSurface = Bone,
    surfaceVariant = Graphite850,
    onSurfaceVariant = Graphite300,
    surfaceContainer = Graphite900,
    surfaceContainerHigh = Graphite850,
    surfaceContainerHighest = Graphite800,
    outline = Graphite700,
    outlineVariant = Graphite800,
    inverseSurface = Bone,
    inverseOnSurface = Ink,
  )

private val LightColorScheme =
  lightColorScheme(
    primary = BrassDeep,
    onPrimary = Color.White,
    primaryContainer = BrassTintLight,
    onPrimaryContainer = BrassDeep,
    secondaryContainer = Steel100,
    onSecondaryContainer = Ink,
    tertiary = BassBlueDeep,
    background = Steel50,
    onBackground = Ink,
    surface = Steel50,
    onSurface = Ink,
    surfaceVariant = Steel100,
    onSurfaceVariant = Graphite500,
    surfaceContainer = Color.White,
    surfaceContainerHigh = Color.White,
    surfaceContainerHighest = Steel100,
    outline = Color(0xFFC3C9CF),
    outlineVariant = Steel100,
    inverseSurface = Graphite900,
    inverseOnSurface = Bone,
  )

/** Colors for the string-count badge by range: bass, regular guitar, extended range. */
@Immutable
data class StringColors(val bass: Pair<Color, Color>, val guitar: Pair<Color, Color>, val extended: Pair<Color, Color>) {
  fun forStrings(n: Int) = if (n <= 5) bass else if (n == 6) guitar else extended
}

private val DarkStringColors = StringColors(BassTintDark to BassBlue, Graphite800 to Bone, BrassTintDark to Brass)
private val LightStringColors = StringColors(BassTintLight to BassBlueDeep, Steel100 to Ink, BrassTintLight to BrassDeep)

val LocalStringColors = staticCompositionLocalOf { DarkStringColors }

@Composable
fun TabFinderTheme(darkTheme: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
  CompositionLocalProvider(LocalStringColors provides if (darkTheme) DarkStringColors else LightStringColors) {
    MaterialTheme(colorScheme = if (darkTheme) DarkColorScheme else LightColorScheme, typography = Typography, content = content)
  }
}
