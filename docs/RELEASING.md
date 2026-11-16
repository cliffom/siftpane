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

Configure the `macos-signing` GitHub environment with appropriate required reviewers and these secrets before dispatching trusted reviewed code:

| Secret | Purpose |
| --- | --- |
| `SIFTPANE_CERTIFICATE_P12_BASE64` | Base64 Developer ID Application certificate/private-key export |
| `SIFTPANE_CERTIFICATE_PASSWORD` | Password protecting that export |
| `SIFTPANE_SIGN_IDENTITY` | Exact Developer ID Application identity |
| `SIFTPANE_NOTARY_KEY_BASE64` | Base64 App Store Connect API private key |
| `SIFTPANE_NOTARY_KEY_ID` | API key ID |
| `SIFTPANE_NOTARY_ISSUER` | Team API issuer ID |

The signing script creates a private temporary keychain/key files and deletes them on exit. It does not touch a user's login keychain or persist signing secrets in artifacts. Missing credentials fail before signing; there is no automatic unsigned fallback. Do not dispatch credentialed signing from untrusted revisions. The manual workflow's environment/credentials must be configured by the repository operator; this PR does not create secrets, change account permissions or claim a successful notarization.

Before publication, run the signed workflow against the exact authorized release revision, download and verify the DMG/checksum, inspect the enclosed REVISION and binary version, exercise browser-downloaded Gatekeeper installation and record the actual supported-host Keychain/terminal/live checks in [0.8 acceptance](ACCEPTANCE-0.8.md). Check the notary log, including warnings. If the release merge changes the source revision, rebuild from that merge before attaching artifacts. Do not reuse a candidate from another commit. Source installs should be checked in a temporary GOBIN. Issues #32/#33 stay open until their corresponding signed/host/live evidence is accepted.
