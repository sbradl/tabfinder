package dev.tabsync.tabfinder

import java.nio.ByteBuffer
import java.nio.ByteOrder

/** Synthetic tab files: Guitar Pro 3 with tunings and a tempo, TuxGuitar 1 headers, junk. */
object Fixture {
  private fun gp3(title: String, artist: String, album: String, tempo: Int, vararg tracks: Pair<String, List<Int>>): ByteArray {
    val b = ByteBuffer.allocate(64 * 1024).order(ByteOrder.LITTLE_ENDIAN)
    fun byteSize(s: String, size: Int) {
      b.put(s.length.toByte())
      b.put(s.toByteArray().copyOf(size))
    }
    fun intByteSize(s: String) {
      b.putInt(s.length + 1)
      byteSize(s, s.length)
    }
    byteSize("FICHIER GUITAR PRO v3.00", 30)
    listOf(title, "", artist, album, "", "", "", "").forEach(::intByteSize)
    b.putInt(0).put(0).putInt(tempo).putInt(0)
    repeat(64) { b.putInt(0).put(ByteArray(8)) }
    b.putInt(0).putInt(tracks.size)
    for ((name, strings) in tracks) {
      b.put(0)
      byteSize(name, 40)
      b.putInt(strings.size)
      for (i in 0 until 7) b.putInt(strings.getOrElse(i) { 0 })
      b.putInt(1).putInt(1).putInt(0).putInt(0).putInt(0).putInt(0)
    }
    return b.array().copyOf(b.position())
  }

  val dropC6 = "G" to listOf(62, 57, 53, 48, 43, 36)
  val eStd6 = "G" to listOf(64, 59, 55, 50, 45, 40)
  val custom6 = "G" to listOf(62, 59, 55, 50, 43, 38)
  val bStd7 = "G7" to listOf(64, 59, 55, 50, 45, 40, 35)
  val bass4 = "B" to listOf(43, 38, 33, 28)

  /** Seven readable tabs and one broken one. */
  val library: Map<String, ByteArray> =
    mapOf(
      "Soilbed Quartet/Glass Orchard/Brass Kettle.gp3" to gp3("Brass Kettle", "Soilbed Quartet", "Glass Orchard", 190, dropC6, bass4),
      "Amber Marsh/Tide of Lanterns/First Frost.gp3" to gp3("First Frost", "Amber Marsh", "Tide of Lanterns", 120, eStd6),
      "Amber Marsh/Tide of Lanterns/Where Rivers Seem to Rest.gp3" to gp3("Where Rivers Seem to Rest", "Amber Marsh", "Tide of Lanterns", 125, custom6),
      "Inkwell Flamingos/Mossman/Mirage.gp3" to gp3("Mirage", "Inkwell Flamingos", "Mossman", 140, dropC6),
      "INKWELL FLAMINGOS/Mossman/Paper Ride.gp3" to gp3("Paper Ride", "INKWELL FLAMINGOS", "Mossman", 100, bStd7),
      "Merrowgate/Catch Fortyone/Compass Lost.gp3" to gp3("Compass Lost", "Merrowgate", "Catch Fortyone", 110, bStd7),
      // Named for something else than its content, so TuxGuitar gets a copy under the right name.
      "Gorsewick/Gravel Hymns/Quartz.gpx.crdownload" to gp3("Quartz", "Gorsewick", "Gravel Hymns", 125, eStd6),
      "Broken/garbled.gp5" to "not a tab".toByteArray(),
    )

  const val SONGS = 8
  const val UNREADABLE = 1
}
