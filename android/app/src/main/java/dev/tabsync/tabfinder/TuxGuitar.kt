package dev.tabsync.tabfinder

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import androidx.core.content.FileProvider
import dev.tabsync.tabfinder.data.Song
import java.io.File

object TuxGuitar {
  const val PACKAGE = "app.tuxguitar.android.application"

  /** Opens the song in TuxGuitar via a cache copy named [Song.openAs], shared through FileProvider. */
  fun open(context: Context, root: String, song: Song) {
    val dir = File(context.cacheDir, "open").apply {
      deleteRecursively()
      mkdirs()
    }
    val copy = File(root, song.path).copyTo(File(dir, song.openAs))
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.files", copy)
    val intent =
      Intent(Intent.ACTION_VIEW)
        .setDataAndType(uri, "application/octet-stream")
        .setPackage(PACKAGE)
        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    try {
      context.startActivity(intent)
    } catch (e: ActivityNotFoundException) {
      throw IllegalStateException("TuxGuitar is not installed", e)
    }
  }
}
