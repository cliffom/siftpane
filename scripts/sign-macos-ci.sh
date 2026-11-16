#!/bin/bash
# Import signing material into a disposable keychain on an ephemeral CI runner.
set -euo pipefail
: "${SIFTPANE_CERTIFICATE_P12_BASE64:?Missing Developer ID Application certificate}"
: "${SIFTPANE_CERTIFICATE_PASSWORD:?Missing certificate password}"
: "${SIFTPANE_SIGN_IDENTITY:?Missing signing identity}"
: "${SIFTPANE_NOTARY_KEY_BASE64:?Missing App Store Connect API key}"
: "${SIFTPANE_NOTARY_KEY_ID:?Missing API key ID}"
: "${SIFTPANE_NOTARY_ISSUER:?Missing API issuer}"
sign_work=$(mktemp -d "${TMPDIR:-/tmp}/siftpane-sign.XXXXXX")
export SIFTPANE_SIGN_KEYCHAIN="$sign_work/signing.keychain-db"
export SIFTPANE_NOTARY_KEY_PATH="$sign_work/notary.p8"
cleanup() {
  /usr/bin/security delete-keychain "$SIFTPANE_SIGN_KEYCHAIN" >/dev/null 2>&1 || true
  rm -rf "$sign_work"
}
trap cleanup EXIT
umask 077
printf '%s' "$SIFTPANE_CERTIFICATE_P12_BASE64" | /usr/bin/base64 -D > "$sign_work/certificate.p12"
printf '%s' "$SIFTPANE_NOTARY_KEY_BASE64" | /usr/bin/base64 -D > "$SIFTPANE_NOTARY_KEY_PATH"
keychain_password=$(/usr/bin/openssl rand -hex 32)
/usr/bin/security create-keychain -p "$keychain_password" "$SIFTPANE_SIGN_KEYCHAIN"
/usr/bin/security set-keychain-settings -lut 21600 "$SIFTPANE_SIGN_KEYCHAIN"
/usr/bin/security unlock-keychain -p "$keychain_password" "$SIFTPANE_SIGN_KEYCHAIN"
/usr/bin/security import "$sign_work/certificate.p12" -k "$SIFTPANE_SIGN_KEYCHAIN" -P "$SIFTPANE_CERTIFICATE_PASSWORD" -T /usr/bin/codesign >/dev/null
/usr/bin/security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$keychain_password" "$SIFTPANE_SIGN_KEYCHAIN" >/dev/null
bash scripts/package-macos.sh --signed "${1:-dist}"
