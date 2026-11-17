# Releases

The authoritative application version is `internal/version/VERSION`, embedded by `internal/version` so checkout builds and `go install` have the same `siftpane --version` identity. Versions use semantic versioning with `v`-prefixed Git tags. Before 1.0, compatible features increment minor, fixes increment patch, and breaking changes increment minor with migration notes. Reuse a suitable prepared, unpublished version. Behavior-preserving refactors and docs-only changes do not need a dedicated release.

`0.1.0` (initial release) and `0.2.0` (local unread title) were published on 2026-11-14. `0.2.1` (background refresh scheduling fix) was also published on 2026-11-14. The `0.3.0` release adds persistent display metadata/name caching, immediate message pages with deferred names, and stable startup loading. The release notes are in [CHANGELOG.md](../CHANGELOG.md). Source builds before the first versioned release have no runtime version identity.

Version `0.3.0` is published. The `0.4.0` release adds sidebar/footer cleanup, a scrollable solid-border help modal, and a bottom-left loading status bar. It is a compatible UI feature release; no migration is required.

When authorized to release:

1. Check the PR, exact head, review findings, and all CI runs. Run `make check` and review the changed UI at desktop and small sizes. Keep live Slack and terminal limitations explicit.
2. Verify the prepared version is unpublished, matches the intended tag, and appears in the changelog. Build and install into temporary locations, check `--version`, and exercise offline snapshots. Do not overwrite a user's installed binary.
3. Merge the verified PR through GitHub. Wait for post-merge CI and tag the verified merge commit, never an unrelated later branch tip. Create an annotated `vVERSION` tag and push only that tag.
4. Publish a GitHub release with focused changelog notes when explicitly authorized. For 0.8, attach only the verified signed/notarized macOS artifacts prepared from the exact released revision using the workflow below; never substitute unsigned CI candidates. Go source installation at the tag must continue to work. Actual host/live acceptance and publication remain distinct.

Release preparation belongs in the implementation PR. Merging, tags, and publication require explicit release authorization. No credentials, personal configuration, or OS keyring state belong in release artifacts.

Version `0.4.0` is published. The `0.5.0` release adds the compatible attention workflow, including the user `search:read` scope in the manifest and account-isolated attention state. Existing installations must reinstall/re-authorize and update the stored user token to enable discovery; manual use remains available without the added grant. The operator confirmed the live acceptance release gate on 2026-11-15; [the evidence record](ATTENTION-DISCOVERY.md) distinguishes that confirmation from assistant-run checks and retains the search limitations. Issue/milestone state is managed separately from tag/publication.

Version `0.5.0` is published. Version **0.6.0** adds compatible optional broadcasts, explicit local thread following and opt-in recent content caching. Existing account state gains optional fields; 0.5 handled message identities stay valid. Older versions cannot interpret new pending broadcast/reply reasons; preserve attention state when downgrading and use the newer version rather than deleting it. New sources default off, existing direct mentions/Kept remain unchanged, no additional scopes or reinstall are required beyond the existing 0.5 `search:read` grant. The operator confirmed the 0.6.0 live acceptance release gate on 2026-11-15. The [evidence record](ATTENTION-DISCOVERY.md#06-extension-evidence) distinguishes that confirmation from assistant-run offline checks; search, physical-terminal and cross-platform limitations remain. Issue/milestone state is managed separately from publication.


Versions `0.6.0` and **0.7.0** are published. The 0.7 release added compatible multi-account and participation features. Re-authorize existing installations for the two reaction scopes and save replacement tokens per account. Back up configuration before legacy preference migration. Startup and login both identify/preserve the legacy owner before adding another account; an unidentifiable old token requires explicit legacy-slot removal before retry. Older clients cannot interpret account preferences. The operator confirmed functionality and release acceptance on 2026-11-15; see the verification record. Assistant-run checks remain separately attributed, with bounded discovery, physical-terminal and cross-platform limitations retained.


## macOS 0.8 candidates and signing

**0.8.0** is prepared and unpublished: compatible in-session credential recovery, read/pagination hardening, setup diagnostics and macOS packaging warrant a minor version. No new persistence schema, scopes, OAuth grant or migration beyond 0.7 is introduced. The supported download target is Apple silicon on macOS 26+. Additional platforms are deferred.

`bash scripts/package-macos.sh --unsigned OUTPUT` builds an explicitly unsigned development archive plus checksum, version-checks the executable and inspects its arm64 architecture/macOS 26 minimum. macOS CI runs this without credentials. It is not a distributable signed release.

The manually dispatched **Signed macOS candidate** workflow runs on `macos-26`, checks the chosen revision, then builds a Developer ID Application-signed executable inside a signed/notarized/stapled DMG. It verifies codesign, notarization status/log, stapler and Gatekeeper before uploading candidate artifacts. It does not tag, publish a release or attach files to one. Official [GitHub runner labels](https://docs.github.com/en/actions/how-tos/write-workflows/choose-where-workflows-run/choose-the-runner-for-a-job) identify `macos-26` as arm64. Apple's [custom notarization workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow) explains container tickets and the inability to staple standalone binaries; hence the DMG distribution. These references were checked on 2026-11-15.

The credential setup follows [Bambustand’s release workflow](https://github.com/cliffom/bambustand/blob/main/.github/workflows/release.yml): repository Actions secrets, an isolated runner Keychain, an app-specific-password notarization profile, and always-run cleanup. A dedicated App Store Connect API key is not required. Configure these secrets in **cliffom/siftpane**, independently of Bambustand:

| Secret | Purpose |
| --- | --- |
| `MACOS_CERTIFICATE_P12_BASE64` | Base64 password-protected Developer ID Application certificate/private-key export |
| `MACOS_CERTIFICATE_PASSWORD` | Fresh password protecting that export |
| `MACOS_SIGN_IDENTITY` | Exact Developer ID Application identity fingerprint or name |
| `APPLE_ID` | Your Developer Apple account email |
| `APPLE_TEAM_ID` | Developer team ID matching the signing identity |
| `APPLE_APP_SPECIFIC_PASSWORD` | New Apple app-specific password labeled for Siftpane notarization |

Use a new Siftpane app-specific password, created at [Apple Account → Sign-In and Security → App-Specific Passwords](https://support.apple.com/en-us/102654). The existing Apple ID, team and Developer ID Application certificate can be reused as the publisher identity. Export the identity with its private key from Keychain Access as a `.p12` protected with a fresh export password; keep that file outside the repository. Do not reuse Bambustand’s app-specific or export password. Repository secrets have the same names but separate values and storage. GitHub cannot return an existing secret’s value, so its secret list cannot be used to copy the Bambustand credentials.

Set the certificate through stdin and use the hidden gh prompts for passwords:

```sh
base64 -i /path/to/siftpane-signing.p12 | gh secret set MACOS_CERTIFICATE_P12_BASE64 --repo cliffom/siftpane
gh secret set MACOS_CERTIFICATE_PASSWORD --repo cliffom/siftpane
gh secret set MACOS_SIGN_IDENTITY --repo cliffom/siftpane
gh secret set APPLE_ID --repo cliffom/siftpane
gh secret set APPLE_TEAM_ID --repo cliffom/siftpane
gh secret set APPLE_APP_SPECIFIC_PASSWORD --repo cliffom/siftpane
```

The Python setup helper is adapted from Bambustand and runs only on a macOS GitHub Actions runner. It validates secrets, imports the selected identity into a private temporary Keychain, checks the identity, and stores the **siftpane-notary** profile in the same Keychain. Only `MACOS_NOTARY_PROFILE` and `MACOS_NOTARY_KEYCHAIN` paths/references are exported to subsequent steps. No password is written to GITHUB_ENV or logs. The decoded export is removed immediately; always-run cleanup restores the original Keychain search list and deletes temporary credentials. Missing credentials or notarization failure stop packaging without an unsigned fallback. Offline synthetic tests exercise setup, failures, secret redaction and cleanup; ordinary PR CI has no signing secrets.

For a local notarized candidate, use the existing Developer ID identity in your Mac’s Keychain and store the **new** app-specific password interactively:

```sh
xcrun notarytool store-credentials siftpane-notary --team-id YOUR_TEAM_ID
MACOS_SIGN_IDENTITY='Developer ID Application: Your Name (TEAMID)' \
MACOS_NOTARY_PROFILE=siftpane-notary \
bash scripts/package-macos.sh --signed /path/to/candidate-output
```

`notarytool` prompts for the Apple ID and password; keep them out of chat, shell history and repository files. `MACOS_NOTARY_KEYCHAIN` is optional locally and explicit on CI. CLI signatures use stable identifier **com.suitepotato.siftpane**, hardened runtime and a secure timestamp. Both executable and DMG must meet Apple’s Developer ID Application signing requirement. Apple must accept the submission; its JSON report/log is retained in ignored candidate-output/notarization, with only those allowlisted reports uploaded as a one-day diagnostic artifact. The DMG is stapled and validated, Gatekeeper assesses it, and the standalone CLI’s notarization record is verified before checksums are generated.

The manual signing workflow must first exist on the default branch before GitHub will dispatch it, as [GitHub documents](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow). Adding the workflow to a PR does not register it, and local credentials do not configure Actions. Once registered, dispatch the trusted reviewed branch with `gh workflow run macos-signing.yml --repo cliffom/siftpane --ref BRANCH`. This workflow creates candidate artifacts only; it does not publish a release. No merge or default-branch write is implied by preparing signing.

Before publication, run the signed workflow against the exact authorized release revision, download and verify the DMG/checksum, inspect the enclosed REVISION and binary version, exercise browser-downloaded Gatekeeper installation and record the actual supported-host Keychain/terminal/live checks in [0.8 acceptance](ACCEPTANCE-0.8.md). Check the notary log, including warnings. If the release merge changes the source revision, rebuild from that merge before attaching artifacts. Do not reuse a candidate from another commit. Source installs should be checked in a temporary GOBIN. Issues #32/#33 stay open until their corresponding signed/host/live evidence is accepted.


A local **signed, Apple-accepted, stapled 0.8.0 candidate** from source `6d0234e` has passed Gatekeeper, checksum and read-only mounted CLI checks; see [the exact receipt](VERIFICATION.md#local-signed-and-notarized-080-candidate). This is local candidate verification, not a published release or credentialed Actions run. Keep the source revision attached to the evidence and rebuild from the exact authorized release revision before publishing. The local siftpane-notary profile is configured; all six repository secrets are configured for the credentialed CI run.
