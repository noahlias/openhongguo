#!/usr/bin/env bash
# Packs target/release/ohg-desktop into openhongguo.app with libmpv, ffmpeg
# (for HEIC covers) and every Homebrew library they load, then into a .dmg.
# Needs Homebrew's mpv, ffmpeg and dylibbundler.
#
#   desktop/scripts/bundle-macos.sh <out-dir> <dmg-name>
set -euo pipefail

out=$1
dmg=$2
here=$(cd "$(dirname "$0")/.." && pwd)
stage="$out/stage"
app="$stage/openhongguo.app"
bin="$app/Contents/MacOS"
libs="$app/Contents/Frameworks"
brew_prefix=$(brew --prefix)

rm -rf "$stage"
mkdir -p "$bin" "$libs"
cp "$here/target/release/ohg-desktop" "$bin/"
cp "$brew_prefix/bin/ffmpeg" "$bin/"
cp "$here/assets/Info.plist" "$app/Contents/"
chmod u+w "$bin/"*

dylibbundler -od -b -cd -ns \
  -x "$bin/ohg-desktop" -x "$bin/ffmpeg" \
  -d "$libs" -p @executable_path/../Frameworks/ \
  -s "$brew_prefix/lib"

# Every Mach-O must load only the system and the bundle; anything else
# (a Homebrew path, an unresolved @rpath) fails on a Mac without Homebrew.
bad=0
for f in "$bin"/* "$libs"/*; do
  while read -r dep; do
    case "$dep" in
      /usr/lib/* | /System/*) ;;
      @executable_path/../Frameworks/*)
        [ -e "$libs/${dep#@executable_path/../Frameworks/}" ] || { echo "missing in bundle: $f -> $dep"; bad=1; } ;;
      *) echo "outside the bundle: $f -> $dep"; bad=1 ;;
    esac
  done < <(otool -L "$f" | tail -n +2 | awk '{print $1}')
done
# A library built for a newer macOS than Info.plist promises fails to load
# there with "Library not loaded" instead of a clear message.
need=$(/usr/libexec/PlistBuddy -c "Print LSMinimumSystemVersion" "$app/Contents/Info.plist")
for f in "$bin"/* "$libs"/*; do
  minos=$(otool -l "$f" | awk '/LC_BUILD_VERSION/ {b = 1} b && $1 == "minos" {print $2; exit}')
  if [ -n "$minos" ] && [ "$(printf '%s\n' "$minos" "$need" | sort -V | tail -1)" != "$need" ]; then
    echo "needs macOS $minos, newer than $need: $f"
    bad=1
  fi
done
[ "$bad" = 0 ] || exit 1

# Rewritten install names invalidate signatures, and arm64 runs only signed code.
codesign --force --sign - "$libs"/*.dylib
codesign --force --sign - "$bin/ffmpeg"
codesign --force --sign - "$app"

ln -s /Applications "$stage/Applications"
cp "$here/../LICENSE" "$stage/"
hdiutil create -volname openhongguo -srcfolder "$stage" -ov -format UDZO "$out/$dmg"
