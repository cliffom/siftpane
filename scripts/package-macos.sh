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
  : "${MACOS_SIGN_IDENTITY:?Set the Developer ID Application signing identity}"
  : "${MACOS_NOTARY_PROFILE:?Set the notarytool Keychain profile name}"
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
/usr/bin/lipo "$work/payload/siftpane" -verify_arch arm64
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
  signing_options=(--timestamp)
  notary_credentials=(--keychain-profile "$MACOS_NOTARY_PROFILE")
  if [[ -n "${MACOS_NOTARY_KEYCHAIN:-}" ]]; then
    signing_options+=(--keychain "$MACOS_NOTARY_KEYCHAIN")
    notary_credentials+=(--keychain "$MACOS_NOTARY_KEYCHAIN")
  fi
  developer_id_requirement='anchor apple generic and certificate leaf[field.1.2.840.113635.100.6.1.13] exists'
  /usr/bin/codesign --force --sign "$MACOS_SIGN_IDENTITY" "${signing_options[@]}" --identifier com.suitepotato.siftpane --options runtime "$work/payload/siftpane"
  /usr/bin/codesign --verify --strict --verbose=2 -R="$developer_id_requirement" "$work/payload/siftpane"
  /usr/bin/hdiutil create -volname "Siftpane $version" -srcfolder "$work/payload" -format UDZO "$work/$stem.dmg"
  /usr/bin/codesign --sign "$MACOS_SIGN_IDENTITY" "${signing_options[@]}" "$work/$stem.dmg"
  /usr/bin/codesign --verify --strict -R="$developer_id_requirement" "$work/$stem.dmg"
  /usr/bin/hdiutil verify "$work/$stem.dmg"
  mkdir -p "$output/notarization"
  # Persist only Apple's JSON reports, never credential files or raw auth stderr.
  if ! xcrun notarytool submit "$work/$stem.dmg" "${notary_credentials[@]}" --wait --timeout 30m --output-format json > "$output/notarization/submission.json" 2> "$work/notary-stderr.txt"; then
    echo "Notarization failed or is still processing; inspect $output/notarization/submission.json before resubmitting." >&2
    exit 1
  fi
  submission=$(/usr/bin/plutil -extract id raw -o - "$output/notarization/submission.json")
  if ! xcrun notarytool log "$submission" "${notary_credentials[@]}" "$output/notarization/log.json" > "$work/notary-log-output.txt" 2>&1; then
    echo 'Cannot retrieve the notarization log; inspect the submission ID before resubmitting.' >&2
    exit 1
  fi
  /usr/bin/plutil -extract status raw -o - "$output/notarization/submission.json" | /usr/bin/grep -qx Accepted
  if ! /usr/bin/osascript -l JavaScript scripts/check-notary-log.js "$output/notarization/log.json"; then
    echo 'Notarization returned issues; inspect the log before distributing.' >&2
    exit 1
  fi
  xcrun stapler staple "$work/$stem.dmg"
  xcrun stapler validate "$work/$stem.dmg"
  /usr/sbin/spctl --assess --type open --context context:primary-signature --verbose=2 "$work/$stem.dmg"
  /usr/bin/codesign --verify --strict --check-notarization -R=notarized "$work/payload/siftpane"
else
  printf '%s\n' 'UNSIGNED DEVELOPMENT CANDIDATE. Not a notarized release download.' > "$work/payload/UNSIGNED.txt"
  /usr/bin/tar -czf "$work/$stem.tar.gz" -C "$work/payload" .
fi
(cd "$work" && /usr/bin/shasum -a 256 "$stem.$extension" > "$stem.SHA256SUMS")
mv "$work/$stem.$extension" "$work/$stem.SHA256SUMS" "$output/"
echo "Prepared $output/$stem.$extension; nothing published."
