# Releases

The authoritative application version is `internal/version/VERSION`, embedded by `internal/version` so checkout builds and `go install` have the same `siftpane --version` identity. Versions use semantic versioning with `v`-prefixed Git tags. Before 1.0, compatible features increment minor, fixes increment patch, and breaking changes increment minor with migration notes. Reuse a suitable prepared, unpublished version. Behavior-preserving refactors and docs-only changes do not need a dedicated release.

`0.1.0` (initial release) and `0.2.0` (local unread title) were published on 2026-11-14. `0.2.1` (background refresh scheduling fix) was also published on 2026-11-14. The `0.3.0` release adds persistent display metadata/name caching, immediate message pages with deferred names, and stable startup loading. The release notes are in [CHANGELOG.md](../CHANGELOG.md). Source builds before the first versioned release have no runtime version identity.

When authorized to release:

1. Check the PR, exact head, review findings, and all CI runs. Run `make check` and review the changed UI at desktop and small sizes. Keep live Slack and terminal limitations explicit.
2. Verify the prepared version is unpublished, matches the intended tag, and appears in the changelog. Build and install into temporary locations, check `--version`, and exercise offline snapshots. Do not overwrite a user's installed binary.
3. Merge the verified PR through GitHub. Wait for post-merge CI and tag the verified merge commit, never an unrelated later branch tip. Create an annotated `vVERSION` tag and push only that tag.
4. Publish a GitHub source release with focused notes from the changelog, marking the initial release notes as released in the next appropriate documentation update. There is no binary packaging/publishing automation in this repository; do not invent signed binaries or cross-platform validation. Go source installation at the tag must work with the existing module/toolchain requirements.

Release preparation belongs in the implementation PR. Merging, tags, and publication require explicit release authorization. No credentials, personal configuration, or OS keyring state belong in release artifacts.
