package dev.tabsync.tabfinder.ui

import androidx.compose.ui.graphics.Color
import dev.tabsync.tabfinder.theme.Graphite950
import dev.tabsync.tabfinder.theme.Steel50
import org.junit.Assert.assertTrue
import org.junit.Test

/** The meter's colors, as in the desktop app (cmd/tabfinder TestLevelColor). */
class LevelColorTest {
  private val backgrounds = mapOf("dark" to Graphite950, "light" to Steel50)

  @Test
  fun `green for 1, red for 10, redder at each level`() {
    for ((name, bg) in backgrounds) {
      assertTrue("$name: level 1 is ${levelColor(1, bg)}", levelColor(1, bg).run { green > red })
      assertTrue("$name: level 10 is ${levelColor(10, bg)}", levelColor(10, bg).run { red > green })
      for (l in 2..10) {
        val a = levelColor(l - 1, bg)
        val b = levelColor(l, bg)
        assertTrue("$name: level $l isn't redder than ${l - 1}", a != b && b.green - b.red <= a.green - a.red)
      }
    }
  }

  @Test
  fun `stands out against the background`() {
    for ((name, bg) in backgrounds) {
      for (l in 1..10) {
        val r = contrast(levelColor(l, bg), bg)
        assertTrue("$name: level $l has a contrast of $r, want 3 or more (WCAG, graphics)", r >= 3)
      }
    }
  }

  @Test
  fun `contrast of black and white`() {
    assertTrue(contrast(Color.Black, Color.White) in 20.9..21.1)
    assertTrue(contrast(Color.White, Color.White) == 1.0)
  }
}
