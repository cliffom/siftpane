# Install and upgrade

The 0.8 download target is **Apple silicon (arm64), macOS 26 or later**. Intel macOS, Linux and Windows are outside the supported download matrix. Source builds remain possible but do not establish platform support. The 0.8 binary and signing workflows are prepared; no 0.8 signed download is published yet.

## Download installation

After an authorized 0.8 release publishes its artifacts, download the matching `siftpane-VERSION-macos-arm64.dmg` and `siftpane-VERSION-macos-arm64.SHA256SUMS` from that release. In the download directory:

```sh
shasum -a 256 -c siftpane-VERSION-macos-arm64.SHA256SUMS
hdiutil attach siftpane-VERSION-macos-arm64.dmg
mkdir -p "$HOME/.local/bin"
install -m 755 "/Volumes/Siftpane VERSION/siftpane" "$HOME/.local/bin/siftpane"
hdiutil detach "/Volumes/Siftpane VERSION"
"$HOME/.local/bin/siftpane" --version
"$HOME/.local/bin/siftpane" --demo
```

Replace VERSION with the actual release version. Add `$HOME/.local/bin` to your shell PATH if needed. The signed distribution is a Developer ID Application-signed command-line executable inside a notarized, stapled disk image. Keep Gatekeeper enabled. A checksum verifies consistency with the published checksum file; it does not replace the Developer ID signature. See [Apple's distribution workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow).

Unsigned `*-unsigned.tar.gz` archives are development candidates from CI/local builds, explicitly marked `UNSIGNED.txt`. They are not the signed release download and have no notarization claim. The source workflow requires Go 1.26.7 or newer; see the README.

## Connect and recover

Follow [Slack setup](SLACK.md) to create/install an internal app, including any administrator approval. Run `siftpane --login`, then `siftpane`. Choose conversations with `p`; sources stay off until explicitly enabled. For an unavailable credential store, unlock macOS Keychain or provide the documented environment override; no plaintext fallback is written.

For a saved account that expires, is revoked, or needs new grants:

1. Reinstall/re-authorize the internal app in Slack when necessary and copy its replacement **User OAuth Token**.
2. In another terminal, run `siftpane --login --account WORKSPACE_ID/USER_ID` (include the same `--config PATH` when using a custom configuration). Paste into the hidden prompt. This validates identity and changes only the existing credential-store slot; it never saves a competing preferences snapshot.
3. Return to the running Siftpane, press Esc to leave composing/pickers, then **Ctrl+R** from navigation. Reload checks the same workspace/user before adoption. Drafts, failed sends and scroll state remain in this process. `v` shows refresh/recovery details.

A wrong-account token is refused. A failed validation/store write leaves recovery available. In-flight sends use their original service and may finish; credential replacement does not resend them. Inspect Slack before explicitly retrying uncertain delivery. Environment-override sessions can revalidate only the inherited process environment token; changing the parent shell environment cannot replace it. To use a different environment token, copy drafts before restarting, or use a registered account for in-session replacement. No environment credential is persisted automatically.

## Upgrade from 0.7

Copy your configuration before upgrading and exit the old process after sending or copying any drafts. Drafts/outbox remain memory-only. Install the new binary in your chosen path and check `--version`; do not run old/new clients concurrently against one config. Existing registered identities, preferences, durable dismissals/follows and opted-in message-cache consent retain their formats. No new Slack scopes or OAuth model are introduced by 0.8. See [configuration](CONFIGURATION.md) for paths and historical 0.7 migration restrictions.

Physical-terminal, Keychain and live Slack acceptance must be recorded on the supported hosts before declaring the 0.8 release accepted; see [the acceptance protocol](ACCEPTANCE-0.8.md).
