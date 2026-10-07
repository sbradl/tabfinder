package dev.tabsync.tabfinder

import android.os.Environment
import android.os.Looper
import android.provider.DocumentsContract
import androidx.test.ext.junit.runners.AndroidJUnit4
import dev.tabsync.tabfinder.ui.FakeSource
import dev.tabsync.tabfinder.ui.MainViewModel
import dev.tabsync.tabfinder.ui.song
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Shadows.shadowOf

/** E-AND-11: the folder picker's answer (MainScreenTest has the new folder's scan). */
@RunWith(AndroidJUnit4::class)
class FolderPickTest {
  private val source = FakeSource("/old").apply { saved = listOf(song("One")) } // a saved scan: none at the start
  private val vm = MainViewModel(source)

  private fun idle() = shadowOf(Looper.getMainLooper()).idle()

  @Test
  fun aFolderOnTheDeviceStorageBecomesTheTabFolder() {
    vm.folderPicked(DocumentsContract.buildTreeDocumentUri("com.android.externalstorage.documents", "primary:Music/Tabs"))
    idle()
    val want = File(Environment.getExternalStorageDirectory(), "Music/Tabs").path
    assertEquals(want, vm.state.value.root)
    assertEquals(want, source.root)
    assertEquals(listOf<String?>(want), source.scans)
  }

  @Test
  fun aFolderWithoutAPathGetsAMessage() {
    vm.folderPicked(DocumentsContract.buildTreeDocumentUri("com.example.cloud", "root:Tabs"))
    idle()
    assertEquals("/old", vm.state.value.root)
    assertEquals("Pick a folder on the device's storage", vm.state.value.message)
  }

  @Test
  fun noFolderChangesNothing() {
    vm.folderPicked(null)
    idle()
    assertEquals("/old", vm.state.value.root)
    assertNull(vm.state.value.message)
  }
}
