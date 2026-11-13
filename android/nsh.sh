#!/bin/sh
# nsh.sh fetches the pinned nsh release (the CLI and the Android embed AARs) into android/.nsh,
# checks each download against the release's .sha256, puts the AARs in app/libs and prints the
# CLI's path:  nsh=$(android/nsh.sh)
set -eu

version=v0.5.0
here=$(cd "$(dirname "$0")" && pwd)
dir="$here/.nsh/$version"

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=macos ;;
*) echo "nsh.sh: no nsh CLI for $(uname -s)" >&2; exit 2 ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=x86_64 ;;
arm64 | aarch64) arch=arm64 ;;
*) echo "nsh.sh: no nsh CLI for $(uname -m)" >&2; exit 2 ;;
esac

sum() {
	if command -v sha256sum >/dev/null; then sha256sum "$@"; else shasum -a 256 "$@"; fi
}

fetch() {
	[ -f "$dir/$1" ] && return
	mkdir -p "$dir"
	url="https://github.com/longbridge/nsh/releases/download/$version/$1"
	if ! (cd "$dir" && curl -fsSL --retry 5 -o "$1" "$url" && curl -fsSL --retry 5 -o "$1.sha256" "$url.sha256" &&
		sum -c "$1.sha256" >&2); then
		rm -f "$dir/$1"
		echo "nsh.sh: could not fetch $url" >&2
		exit 1
	fi
}

fetch "nsh-$os-$arch.tar.gz"
fetch nsh-embed-android.zip
[ -x "$dir/nsh/bin/nsh" ] || tar -xzf "$dir/nsh-$os-$arch.tar.gz" -C "$dir"
[ -f "$dir/nsh-embed-android/nsh-dev.aar" ] || unzip -q -o "$dir/nsh-embed-android.zip" -d "$dir"

mkdir -p "$here/app/libs"
cp "$dir/nsh-embed-android/nsh.aar" "$dir/nsh-embed-android/nsh-dev.aar" "$here/app/libs/"
echo "$dir/nsh/bin/nsh"
