package dev.tabsync.tabfinder

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.espresso.intent.Intents
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assume.assumeTrue
import org.junit.Test

/** E-AND-01, the part only a device has: Android's storage access on a fresh install (MainScreenTest has the prompts). */
class AccessTest : DeviceTest() {
  // E-AND-01: needs an app without access, so it only runs with `withoutAccess=true` (see DeviceTest; `mise run test-device`
  // does that). On API 29 the button asks for READ_EXTERNAL_STORAGE: the system's dialog opens, and once Allow is
  // tapped and the app is back, the folder prompt shows. On API 30+ it opens the all-files settings.
  @Test
  fun accessPromptWhenNothingIsGranted() {
    assumeTrue("run with withoutAccess=true on an app that has no access", withoutAccess)
    assertFalse("the app has access already", hasAccess())
    launch()
    compose.onNodeWithText("Allow file access").assertIsDisplayed()
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
      // The button opens this app's all-files settings. Only a fresh install gets to check that: switching the
      // access off from a test kills the app's process, which is the test's too.
      Intents.init()
      try {
        compose.onNodeWithTag("prompt-action").performClick()
        // The recorded intent, not Intents.intended, which waits for window focus (see SmokeTest).
        until { Intents.getIntents().any { it.action == Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION } }
        val intent = Intents.getIntents().single { it.action == Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION }
        assertEquals("package:${context.packageName}", intent.dataString)
      } finally {
        Intents.release()
      }
      return
    }
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
}
