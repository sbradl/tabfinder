package dev.tabsync.tabfinder

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.core.net.toUri
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.viewmodel.compose.viewModel
import dev.tabsync.tabfinder.data.Finder
import dev.tabsync.tabfinder.theme.TabFinderTheme
import dev.tabsync.tabfinder.ui.MainScreen
import dev.tabsync.tabfinder.ui.MainViewModel

class MainActivity : ComponentActivity() {
  override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)

    enableEdgeToEdge()
    setContent {
      TabFinderTheme { Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) { AndroidMainScreen() } }
    }
  }
}

/** The shared screen with Android's file permission, folder picker and TuxGuitar intent. */
@Composable
private fun AndroidMainScreen() {
  val context = LocalContext.current
  val app = context.applicationContext
  val viewModel = viewModel { MainViewModel(Finder(app)) }

  // All-files access is granted in system settings, so re-check whenever we come back.
  var hasAccess by remember { mutableStateOf(hasStorageAccess(context)) }
  LifecycleResumeEffect(Unit) {
    hasAccess = hasStorageAccess(context)
    onPauseOrDispose {}
  }
  val requestRead = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { hasAccess = it }
  val pickFolder =
    rememberLauncherForActivityResult(ActivityResultContracts.OpenDocumentTree()) { uri ->
      if (uri == null) return@rememberLauncherForActivityResult
      Finder.treeToPath(uri)?.let(viewModel::setRoot) ?: viewModel.showMessage("Pick a folder on the device's storage")
    }

  MainScreen(
    viewModel,
    onPickFolder = { pickFolder.launch(null) },
    onOpen = { root, song -> TuxGuitar.open(context, root, song) },
    hasAccess = hasAccess,
    onRequestAccess = {
      if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
        context.startActivity(Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, "package:${context.packageName}".toUri()))
      } else {
        requestRead.launch(Manifest.permission.READ_EXTERNAL_STORAGE)
      }
    },
  )
}

/** All-files access on Android 11+; on 10, legacy storage makes plain read permission enough. */
private fun hasStorageAccess(context: Context) =
  if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) Environment.isExternalStorageManager()
  else context.checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
