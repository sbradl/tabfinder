package dev.tabsync.tabfinder

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import androidx.core.content.FileProvider
import dev.tabsync.tabfinder.data.Song
import java.io.File
import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

object TuxGuitar {
  const val PACKAGE = "app.tuxguitar.android.application"

  /**
   * Opens the song in TuxGuitar via a cache copy named [Song.openAs], shared through FileProvider. The copy is
   * made off the main thread. Returns why the song couldn't be opened, or null.
   */
  suspend fun open(context: Context, root: String, song: Song): String? {
    val copy =
      try {
        withContext(Dispatchers.IO) {
          val dir = File(context.cacheDir, "open").apply {
            deleteRecursively()
            mkdirs()
          }
          File(root, song.path).copyTo(File(dir, song.openAs))
        }
      } catch (e: IOException) {
        return e.message ?: "Couldn't read ${song.path}"
      }
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.files", copy)
    val intent =
      Intent(Intent.ACTION_VIEW)
        .setDataAndType(uri, "application/octet-stream")
        .setPackage(PACKAGE)
        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    return try {
      context.startActivity(intent)
      null
    } catch (_: ActivityNotFoundException) {
      "TuxGuitar is not installed"
    }
  }
}
