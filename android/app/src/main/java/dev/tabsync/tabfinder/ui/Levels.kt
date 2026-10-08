package dev.tabsync.tabfinder.ui

import dev.tabsync.tabfinder.data.Query

/** What a band member plays: [key] is how tabscan names it. */
enum class Role(val key: String, val label: String) {
  DRUMS("drums", "Drums"),
  BASS("bass", "Bass"),
  RHYTHM("rhythm", "Rhythm guitar"),
  LEAD("lead", "Lead guitar");

  companion object {
    fun of(key: String): Role? = entries.firstOrNull { it.key == key }
  }
}

/** The levels of a part: 1 to 10. */
val allLevels = 1..10

/** A role's range of levels in the summary under the fields: "5–7", or "8". */
data class LevelChip(val role: Role, val text: String)

/** The range of levels a role's part must be in, as typed: "5-7", "7-", "-3", "8"; empty for any. */
fun Query.levelText(role: Role): String =
  when (role) {
    Role.DRUMS -> drums
    Role.BASS -> bass
    Role.RHYTHM -> rhythm
    Role.LEAD -> lead
  }

/** The range of levels a role's part must be in: [allLevels] for any, or what can't be read. */
fun Query.level(role: Role): IntRange {
  val text = levelText(role).trim()
  if (text.isEmpty()) return allLevels
  val (lo, hi) = if ('-' in text) text.split("-", limit = 2).let { it[0].trim() to it[1].trim() } else text to text
  val from = if (lo.isEmpty()) allLevels.first else lo.toIntOrNull() ?: return allLevels
  val to = if (hi.isEmpty()) allLevels.last else hi.toIntOrNull() ?: return allLevels
  return maxOf(from, allLevels.first)..minOf(to, allLevels.last)
}

/** The query with a role's range of levels; [allLevels] is no filter. */
fun Query.withLevel(role: Role, range: IntRange): Query {
  val text = if (range.first <= allLevels.first && range.last >= allLevels.last) "" else "${range.first}-${range.last}"
  return when (role) {
    Role.DRUMS -> copy(drums = text)
    Role.BASS -> copy(bass = text)
    Role.RHYTHM -> copy(rhythm = text)
    Role.LEAD -> copy(lead = text)
  }
}

/** A chip per role with a range of levels, in the order of the parts. */
fun Query.chips(): List<LevelChip> =
  Role.entries.mapNotNull { role ->
    val r = level(role)
    when {
      r == allLevels -> null
      r.first == r.last -> LevelChip(role, "${r.first}")
      else -> LevelChip(role, "${r.first}–${r.last}")
    }
  }
