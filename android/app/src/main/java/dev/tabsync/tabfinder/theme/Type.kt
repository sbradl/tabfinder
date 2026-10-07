package dev.tabsync.tabfinder.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import dev.tabsync.tabfinder.R

// Barlow Condensed (amp panel lettering) for titles and labels, system sans for running text,
// monospace for numbers and notes, which line up like tab.

val Mono = FontFamily.Monospace

private val condensed = FontFamily(Font(R.font.barlow_condensed_medium, FontWeight.Medium), Font(R.font.barlow_condensed_semibold, FontWeight.SemiBold))

val Typography =
  Typography(
    titleLarge = TextStyle(fontFamily = condensed, fontWeight = FontWeight.SemiBold, fontSize = 26.sp, lineHeight = 30.sp, letterSpacing = 0.5.sp),
    titleMedium = TextStyle(fontFamily = condensed, fontWeight = FontWeight.SemiBold, fontSize = 20.sp, lineHeight = 24.sp, letterSpacing = 0.2.sp),
    bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 24.sp, letterSpacing = 0.2.sp),
    bodyMedium = TextStyle(fontSize = 14.sp, lineHeight = 20.sp, letterSpacing = 0.2.sp),
    labelLarge = TextStyle(fontFamily = condensed, fontWeight = FontWeight.SemiBold, fontSize = 16.sp, lineHeight = 20.sp, letterSpacing = 0.8.sp),
    labelMedium = TextStyle(fontFamily = condensed, fontWeight = FontWeight.Medium, fontSize = 14.sp, lineHeight = 18.sp, letterSpacing = 1.2.sp),
    labelSmall = TextStyle(fontFamily = Mono, fontWeight = FontWeight.Medium, fontSize = 12.sp, lineHeight = 16.sp),
  )
