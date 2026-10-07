#!/bin/sh
# E-AND-10, the part a test inside the app can't do: kill the app's process in the background and
# come back. Run on the debug build (`mise run test-device` installs it as dev.tabsync.tabfinder.debug,
# next to the real app). It gives the app a small tab folder (the made-up fixture library of
# internal/tab/testdata), checks the list is on screen, goes home, kills the process, reopens the app and
# checks that the list is back and nothing crashed. Filters are not saved across process death (only the list).
# The folder is in shared storage: from Android 11 on, adb can't create folders in another app's Android/data.
set -eu
cd "$(dirname "$0")/.."
PKG=dev.tabsync.tabfinder.debug
ACTIVITY=$PKG/dev.tabsync.tabfinder.MainActivity
DIR=/sdcard/TabFinderProcessDeathTest
SONG="Salt Lamp" # a title in the fixture library

fail() {
  echo "FAIL: $1" >&2
  # What was on screen: the texts of the last dump, and a screenshot (CI uploads it).
  echo "on screen:" >&2
  adb shell cat /sdcard/ui.xml 2>/dev/null | grep -o 'text="[^"]\{1,\}"' | head -40 >&2 || true
  mkdir -p android/app/build && adb exec-out screencap -p > android/app/build/process-death.png || true
  exit 1
}
on_screen() { adb shell "uiautomator dump /sdcard/ui.xml >/dev/null 2>&1; cat /sdcard/ui.xml 2>/dev/null" | grep -q "$1"; }
wait_for() { # wait_for TEXT SECONDS
  i=0
  while [ "$i" -lt "$2" ]; do
    on_screen "$1" && return 0
    sleep 1
    i=$((i + 1))
  done
  return 1
}
cleanup() {
  adb shell am force-stop $PKG
  adb shell "run-as $PKG rm -f shared_prefs/settings.xml files/index.jsonl" || true
  adb shell rm -rf "$DIR" /sdcard/ui.xml
}

adb shell pm list packages | grep -q "package:$PKG\$" || fail "$PKG is not installed (run mise run test-device first)"
trap cleanup EXIT

adb shell am force-stop $PKG
adb shell rm -rf "$DIR"
adb shell mkdir -p "$DIR"
adb push internal/tab/testdata/library/. "$DIR" >/dev/null
# Storage access: the permission up to Android 10, the all-files app op from Android 11 on (as the device tests do).
if [ "$(adb shell getprop ro.build.version.sdk | tr -d '\r')" -le 29 ]; then
  adb shell pm grant $PKG android.permission.READ_EXTERNAL_STORAGE
else
  adb shell appops set $PKG MANAGE_EXTERNAL_STORAGE allow
fi
adb shell "run-as $PKG sh -c 'mkdir -p shared_prefs && rm -f files/index.jsonl && echo \"<?xml version=\\\"1.0\\\" encoding=\\\"utf-8\\\" standalone=\\\"yes\\\"?><map><string name=\\\"root\\\">$DIR</string></map>\" > shared_prefs/settings.xml'"

adb logcat -c
adb shell am start -n $ACTIVITY >/dev/null
wait_for "$SONG" 30 || fail "the list isn't on screen after the first start (no '$SONG')"
adb shell input keyevent KEYCODE_HOME
sleep 1
# `am kill` leaves a process alone that the system doesn't count as cached yet; the debug build lets run-as kill it.
adb shell am kill $PKG
adb shell "pid=\$(pidof $PKG) && run-as $PKG kill -9 \$pid" || true
sleep 1
adb shell pidof $PKG >/dev/null && fail "the process survived the kill"
adb shell am start -n $ACTIVITY >/dev/null
wait_for "$SONG" 30 || fail "the list didn't come back after process death (no '$SONG')"
adb shell pidof $PKG >/dev/null || fail "the app isn't running after the restart"
if adb logcat -d -s AndroidRuntime:E | grep -q "$PKG"; then
  adb logcat -d -s AndroidRuntime:E | tail -20 >&2
  fail "crash in the log"
fi
echo "ok: after process death the app restarted without a crash and shows the list again"
