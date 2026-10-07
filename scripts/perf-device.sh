#!/bin/sh
# E-AND-12: startup and scroll performance of the installed release app on the connected device.
# Install it first with `mise run install` (release build, compiled with `cmd package compile -m speed`).
# Needs a saved scan of about 950 songs; it measures what is there. Nothing is changed in the app
# except that it is stopped and started.
set -eu
PKG=dev.tabsync.tabfinder
START_LIMIT_MS=1000
JANK_LIMIT_PERCENT=5

adb shell pm list packages | grep -q "package:$PKG\$" || { echo "$PKG is not installed" >&2; exit 1; }

adb shell am force-stop $PKG
total=$(adb shell am start -W -n $PKG/.MainActivity | tr -d '\r' | sed -n 's/^TotalTime: *//p')
echo "cold start: ${total} ms (limit $START_LIMIT_MS)"
[ "$total" -lt "$START_LIMIT_MS" ] || { echo "FAIL: cold start too slow" >&2; exit 1; }

sleep 2
adb shell dumpsys gfxinfo $PKG reset >/dev/null
for i in $(seq 1 30); do
  adb shell input swipe 600 1500 600 400 150
  adb shell input swipe 600 400 600 1500 150
done
sleep 1
stats=$(adb shell dumpsys gfxinfo $PKG | tr -d '\r' | sed -n 's/^Janky frames: //p' | head -1)
echo "janky frames: $stats"
percent=$(echo "$stats" | sed -n 's/.*(\([0-9.]*\)%).*/\1/p')
[ -n "$percent" ] || { echo "FAIL: no frame statistics from dumpsys gfxinfo" >&2; exit 1; }
awk -v p="$percent" -v l="$JANK_LIMIT_PERCENT" 'BEGIN { exit !(p < l) }' || { echo "FAIL: ${percent}% janky frames (limit ${JANK_LIMIT_PERCENT}%)" >&2; exit 1; }
echo "ok"
