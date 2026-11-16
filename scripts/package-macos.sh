#!/bin/bash
# Build a release candidate without publishing or touching installed software.
set -euo pipefail

mode=${1:---signed}
output=${2:-dist}
case "$mode" in
  --signed|--unsigned) ;;
  *) echo 'Usage: scripts/package-macos.sh [--signed|--unsigned] [output-directory]' >&2; exit 1 ;;
esac
if [[ $(uname -s) != Darwin || $(uname -m) != arm64 ]]; then
  echo 'Packaging requires an Apple silicon macOS host.' >&2
  exit 1
fi
major=$(/usr/bin/sw_vers -productVersion)
if (( ${major%%.*} < 26 )); then
  echo 'Packaging requires macOS 26 or later.' >&2
  exit 1
fi
if [[ "$mode" == --signed ]]; then
  : "${SIFTPANE_SIGN_IDENTITY:?Set the Developer ID Application signing identity}"
  : "${SIFTPANE_SIGN_KEYCHAIN:?Set the dedicated signing keychain path}"
  : "${SIFTPANE_NOTARY_KEY_PATH:?Set the App Store Connect API key path}"
  : "${SIFTPANE_NOTARY_KEY_ID:?Set the App Store Connect key ID}"
  : "${SIFTPANE_NOTARY_ISSUER:?Set the App Store Connect issuer ID}"
fi
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
if [[ "$mode" == --signed && -n $(git status --porcelain) ]]; then
  echo 'Signed candidates require a clean committed revision.' >&2
  exit 1
fi
version=$(tr -d '\n' < internal/version/VERSION)
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo 'Invalid application version.' >&2
  exit 1
fi
mkdir -p "$output"
output=$(cd "$output" && pwd)
stem="siftpane-${version}-macos-arm64"
extension=dmg
if [[ "$mode" == --unsigned ]]; then
  stem="${stem}-unsigned"
  extension=tar.gz
fi
for name in "$stem.$extension" "$stem.SHA256SUMS"; do
  if [[ -e "$output/$name" ]]; then
    echo "Refusing to overwrite existing artifact: $name" >&2
    exit 1
  fi
done
work=$(mktemp -d "${TMPDIR:-/tmp}/siftpane-package.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir "$work/payload"
export GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=26.0
go build -trimpath -ldflags='-linkmode=external -extldflags=-mmacosx-version-min=26.0' -o "$work/payload/siftpane" ./cmd/siftpane
[[ $("$work/payload/siftpane" --version) == "siftpane $version" ]]
/usr/bin/lipo -verify_arch arm64 "$work/payload/siftpane"
/usr/bin/vtool -show-build "$work/payload/siftpane" > "$work/build-info.txt"
if ! /usr/bin/awk '$1 == "minos" && $2 == "26.0" { found=1 } END { exit !found }' "$work/build-info.txt"; then
  echo 'Binary does not declare the supported macOS 26.0 minimum.' >&2
  exit 1
fi
cp README.md CHANGELOG.md slack-app-manifest.yaml "$work/payload/"
cp -R docs "$work/payload/"
cp -R assets "$work/payload/"
# Preserve the full revision used for the candidate; never include local config.
git rev-parse HEAD > "$work/payload/REVISION"
if [[ -n $(git status --porcelain) ]]; then
  printf '%s\n' 'Uncommitted development worktree; REVISION is only its base.' > "$work/payload/WORKTREE.txt"
fi
if [[ "$mode" == --signed ]]; then
  /usr/bin/codesign --force --sign "$SIFTPANE_SIGN_IDENTITY" --keychain "$SIFTPANE_SIGN_KEYCHAIN" --options runtime --timestamp "$work/payload/siftpane"
  /usr/bin/codesign --verify --strict --verbose=2 "$work/payload/siftpane"
  /usr/bin/hdiutil create -volname "Siftpane $version" -srcfolder "$work/payload" -format UDZO "$work/$stem.dmg"
  /usr/bin/codesign --sign "$SIFTPANE_SIGN_IDENTITY" --keychain "$SIFTPANE_SIGN_KEYCHAIN" --timestamp "$work/$stem.dmg"
  /usr/bin/hdiutil verify "$work/$stem.dmg"
  if ! xcrun notarytool submit "$work/$stem.dmg" --key "$SIFTPANE_NOTARY_KEY_PATH" --key-id "$SIFTPANE_NOTARY_KEY_ID" --issuer "$SIFTPANE_NOTARY_ISSUER" --wait --timeout 30m --output-format json > "$work/notarization.json"; then
    cat "$work/notarization.json" >&2
    exit 1
  fi
  /usr/bin/plutil -extract status raw -o - "$work/notarization.json" | /usr/bin/grep -qx Accepted
  submission=$(/usr/bin/plutil -extract id raw -o - "$work/notarization.json")
  xcrun notarytool log "$submission" --key "$SIFTPANE_NOTARY_KEY_PATH" --key-id "$SIFTPANE_NOTARY_KEY_ID" --issuer "$SIFTPANE_NOTARY_ISSUER" "$work/notary-log.json"
  if ! /usr/bin/osascript -l JavaScript scripts/check-notary-log.js "$work/notary-log.json"; then
    echo 'Notarization returned issues; inspect the log before distributing.' >&2
    cat "$work/notary-log.json" >&2
    exit 1
  fi
  xcrun stapler staple "$work/$stem.dmg"
  xcrun stapler validate "$work/$stem.dmg"
  /usr/sbin/spctl --assess --type open --context context:primary-signature --verbose=2 "$work/$stem.dmg"
else
  printf '%s\n' 'UNSIGNED DEVELOPMENT CANDIDATE. Not a notarized release download.' > "$work/payload/UNSIGNED.txt"
  /usr/bin/tar -czf "$work/$stem.tar.gz" -C "$work/payload" .
fi
(cd "$work" && /usr/bin/shasum -a 256 "$stem.$extension" > "$stem.SHA256SUMS")
mv "$work/$stem.$extension" "$work/$stem.SHA256SUMS" "$output/"
echo "Prepared $output/$stem.$extension; nothing published."
