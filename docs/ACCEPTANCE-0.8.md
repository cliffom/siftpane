# 0.8 controlled daily-use acceptance

Tracks [#33](https://github.com/cliffom/siftpane/issues/33), with installation/signing host evidence in [#32](https://github.com/cliffom/siftpane/issues/32). This is a protocol with partial evidence, not a completed daily-use acceptance report. Earlier 0.5–0.7 operator acceptance remains historical evidence. New 0.8 acceptance is outstanding.

Record the exact candidate revision/checksum, macOS version, Apple silicon model, terminal/font and operator. Target macOS 26 and a current later supported release separately; a single current host is not evidence for every supported OS. Use synthetic labels and private fixture mappings. Never paste tokens, private IDs or message bodies into issues or evidence. Test sends/reactions require explicit operator authorization and controlled targets.

| Case | Observable acceptance | Evidence status |
| --- | --- | --- |
| Fresh downloaded install | Verify checksum, Developer ID, notarization/stapled DMG and Gatekeeper behavior of an actual browser download; install/run version and demo without Go | Local candidate 6d0234e passed signing/notarization/stapling, Gatekeeper assessment, checksum and mounted CLI checks; actual browser download/clean-host install pending |
| New connection | Internal app administrator/grant flow, hidden user token entry, Keychain save/read, empty account, explicit first conversation; no implicit source consent | Pending |
| Upgrade from 0.7 | Preserve account preferences, dismissals/follows and cache controls; confirm no new grant required | Pending |
| Reading | Edit/delete before and at selection, resize desktop/small views, page older history and switch accounts; selected actions target visible context | Pending |
| Large thread | Multiple pages and more than 300 replies, retained root, partial/error states, failed-page retry and bounded cursor recovery where observable | Pending |
| Network/rate limiting | Interrupt and resume reads; current/source errors survive unrelated actions, Retry-After respected when observable; no mutation retry | Pending |
| Credential replacement | Revoke/replace the selected saved account; wrong identity refused; Keychain locked/failure path; reload without losing conversation/thread drafts or recovery | Pending |
| Delayed results | Switch during reads, credential reload and authorized successful/failed/uncertain sends; original account owns completion, inactive accounts stay paused | Pending |
| macOS integration | Real Keychain replacement/logout, browser exact message/account handling and physical terminal 100×30 / 60×20, plus relevant minimum layout | Pending |

Keep unobserved live cursor expiry, live rate limiting and search/indexing limitations explicit even when deterministic offline fixtures pass. Record case-by-case outcomes or clearly attribute operator confirmation, without manufacturing timings or platform coverage. Issues #32/#33 remain open for missing signed/host/live evidence. Actual release publication is separately authorized.


Local signing receipt, exact source revision, submission ID and checksum are in [the verification record](VERIFICATION.md#local-signed-and-notarized-080-candidate). That candidate is retained locally and unpublished. Credentialed CI, actual browser-download acceptance and the remaining Slack/Keychain/host cases above are separate gates.
