#!/usr/bin/env bash
# Packs target/release/ohg-desktop into openhongguo.app with libmpv, ffmpeg
# (for HEIC covers) and every Homebrew library they load, then into a .dmg.
# Needs Homebrew's mpv, ffmpeg, dylibbundler and librsvg, and dmgbuild
# (`pipx install dmgbuild`).
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
res="$app/Contents/Resources"
art="$here/assets/macos"
brew_prefix=$(brew --prefix)

rm -rf "$stage"
mkdir -p "$bin" "$libs" "$res"
cp "$here/target/release/ohg-desktop" "$bin/"
cp "$brew_prefix/bin/ffmpeg" "$bin/"
cp "$here/assets/Info.plist" "$app/Contents/"
cp "$here/../LICENSE" "$res/"
chmod u+w "$bin/"*

iconset="$out/AppIcon.iconset"
rm -rf "$iconset"
mkdir -p "$iconset"
for s in 16 32 128 256 512; do
  rsvg-convert -w "$s" -h "$s" "$art/icon.svg" -o "$iconset/icon_${s}x${s}.png"
  rsvg-convert -w $((s * 2)) -h $((s * 2)) "$art/icon.svg" -o "$iconset/icon_${s}x${s}@2x.png"
done
iconutil -c icns "$iconset" -o "$res/AppIcon.icns"
# dmgbuild picks up background@2x.png beside background.png for Retina.
rsvg-convert -w 600 -h 380 "$art/dmg-background.svg" -o "$out/background.png"
rsvg-convert -w 1200 -h 760 "$art/dmg-background.svg" -o "$out/background@2x.png"

dylibbundler -od -b -cd -ns \
  -x "$bin/ohg-desktop" -x "$bin/ffmpeg" \
  -d "$libs" -p @executable_path/../Frameworks/ \
  -s "$brew_prefix/lib"

rpaths() { otool -l "$1" | awk '$2 == "LC_RPATH" {getline; getline; print $2}'; }

# dylibbundler adds its rpath once per pass over a file, and dyld refuses
# a binary with a duplicate LC_RPATH ("Library not loaded" at launch).
for f in "$bin"/* "$libs"/*; do
  for rp in $(rpaths "$f" | sort | uniq -d); do
    while [ "$(rpaths "$f" | grep -cxF "$rp")" -gt 1 ]; do
      install_name_tool -delete_rpath "$rp" "$f"
    done
  done
done

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
  if [ -n "$(rpaths "$f" | sort | uniq -d)" ]; then
    echo "duplicate LC_RPATH: $f"
    bad=1
  fi
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

dmgbuild -s "$art/dmg.py" -D app="$app" -D icon="$res/AppIcon.icns" \
  -D background="$out/background.png" openhongguo "$out/$dmg"
