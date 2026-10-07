package dev.tabsync.tabfinder

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.espresso.intent.Intents
import androidx.test.espresso.intent.matcher.IntentMatchers.hasAction
import androidx.test.espresso.intent.matcher.IntentMatchers.hasData
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test

/** E-AND-01 to E-AND-03 */
class AccessAndScanTest : DeviceTest() {
  // E-AND-01: needs an app without access, so it only runs with `withoutAccess=true` (see DeviceTest; `mise run test-device`
  // does that). On API 29 the button asks for READ_EXTERNAL_STORAGE: the system's dialog opens, and once Allow is
  // tapped and the app is back, the folder prompt shows.
  @Test
  fun accessPromptWhenNothingIsGranted() {
    assumeTrue("run with withoutAccess=true on an app that has no access", withoutAccess)
    assertFalse("the app has access already", hasAccess())
    launch()
    compose.onNodeWithText("Allow file access").assertIsDisplayed()
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) return // the button's settings screen is the next test
    compose.onNodeWithTag("prompt-action").performClick()
    until(10_000) { focusedWindow().contains("permission", ignoreCase = true) } // the system's permission dialog
    until(10_000) { tapAllow() }
    until(10_000) { compose.onAllNodes(hasText("Choose your tab folder")).fetchSemanticsNodes().isNotEmpty() }
  }

  private fun hasAccess() =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) android.os.Environment.isExternalStorageManager()
    else context.checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED

  /** Taps the permission dialog's Allow button; false while it isn't on screen. */
  private fun tapAllow(): Boolean {
    val root = instrumentation.uiAutomation.rootInActiveWindow ?: return false
    val button =
      listOf("com.android.permissioncontroller", "com.android.packageinstaller")
        .flatMap { root.findAccessibilityNodeInfosByViewId("$it:id/permission_allow_button") }
        .firstOrNull() ?: root.findAccessibilityNodeInfosByText("Allow").firstOrNull { it.isClickable } ?: return false
    return button.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK)
  }

  private fun focusedWindow() = shell("dumpsys window").lines().firstOrNull { "mCurrentFocus" in it } ?: ""

  // E-AND-01: the button on API 30+ opens the all-files settings of this app.
  @Test
  fun allowAccessOpensTheAllFilesSettingsOnAndroid11AndUp() {
    assumeTrue(Build.VERSION.SDK_INT >= Build.VERSION_CODES.R)
    assumeTrue(!withoutAccess)
    shell("appops set ${context.packageName} MANAGE_EXTERNAL_STORAGE deny")
    Intents.init()
    try {
      launch()
      compose.onNodeWithText("Allow file access").assertIsDisplayed()
      compose.onNodeWithTag("prompt-action").performClick()
      Intents.intended(
        org.hamcrest.Matchers.allOf(
          hasAction(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION),
          hasData("package:${context.packageName}"),
        )
      )
    } finally {
      Intents.release()
      shell("appops set ${context.packageName} MANAGE_EXTERNAL_STORAGE allow")
    }
  }

  // E-AND-01: no folder yet.
  @Test
  fun folderPromptWithoutAFolder() {
    launch()
    until(10_000) { compose.onAllNodes(hasTestTag("prompt")).fetchSemanticsNodes().isNotEmpty() }
    compose.onNodeWithText("Choose your tab folder").assertIsDisplayed()
    compose.onNodeWithText("Choose folder").assertIsDisplayed()
  }

  // E-AND-02
  @Test
  fun scanRunsAndTheListAppears() {
    fixture()
    launch()
    // (The snackbar with "N tabs, M unreadable" isn't asserted here: the Compose test clock runs ahead whenever the
    // test waits, so its 4 s are over at once, and holding the clock stops the app's recomposition too.
    // MainViewModelTest covers the message.)
    waitForSongs()
    assertEquals("${Fixture.SONGS} / ${Fixture.SONGS}", counter())
    // tabscan -serve is running as a child of the app, and the scan was saved.
    assertEquals(1, tabscanPids().size)
    assertTrue(index.isFile)
    assertEquals(Fixture.SONGS, index.readLines().count { it.isNotBlank() })
  }

  // E-AND-03
  @Test
  fun restartShowsTheSavedScanWithoutScanning() {
    fixture()
    launch()
    waitForSongs() // the first start scans and saves the index
    scenario!!.close()
    scenario = null
    // The first start scanned; make sure nothing scans on the second.
    killTabscan()
    val saved = index.lastModified()
    assertTrue(index.isFile)
    Thread.sleep(1100)
    launch()
    waitForSongs()
    assertEquals(saved, index.lastModified())
    assertFalse("a scan message appeared", compose.onAllNodes(hasText("tabs, ", substring = true)).fetchSemanticsNodes().any { it.config.toString().contains("unreadable") })
  }
}



