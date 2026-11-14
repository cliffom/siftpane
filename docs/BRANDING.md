# Project artwork

[Project / Slack app icon](../assets/branding/icon.png)

The icon uses a retro terminal frame, a split pane, a command prompt, and two original conversation bubbles. It draws on the pixel-art terminal character of [Bambustand](https://github.com/cliffom/bambustand) with Siftpane's teal, mint, and amber palette. It does not use the Slack glyph or four-color mark. The earlier Slack-like draft was superseded and is not part of this repository.

The generated raster master is a **1254 × 1254 opaque PNG**, with a square, unrounded canvas and safe margins. It serves both as the project mark and as the upload for the Slack app icon. This is pixel-inspired raster artwork, not an editable pixel-grid/vector master.

To use it in Slack, open the app's **Basic Information → Display Information** and upload `assets/branding/icon.png` as the app icon. Slack's [icon API requirements](https://docs.slack.dev/reference/methods/apps.icon.set/) specify dimensions from 512 to 2000 pixels square. Its [design guidance](https://docs.slack.dev/concepts/app-design/) recommends simple artwork that reads at small sizes and leaving the outer corners unrounded so Slack can apply its own crop.

## Generation record

Created with the built-in image-generation tool. The final edit used the superseded terminal draft as image 1 and the local Bambustand `assets/branding/icon-512.png` as image 2, a style reference. Original generation files remain outside the repository; the selected final image is copied into the project above. Keep changes non-destructive until a replacement is chosen.

Final prompt, verbatim:

```text
Use case: logo-brand. Create the revised final Siftpane app icon. Input image 1: edit target, the draft terminal icon. Input image 2: Bambustand style reference, especially its genuinely flat limited-palette pixel artwork. Primary change: COMPLETELY REMOVE the Slack-like four-color hash from the right pane of image 1. Replace it with an original simple symbol of two pixel speech bubbles: one larger mint/teal bubble slightly upper-left and one smaller amber bubble slightly lower-right, both with chunky square pixel tails and plain dark horizontal message strokes. These are abstract conversations, no borrowed company logo. Make the bubbles balanced and clearly separate, readable at a 36px app-avatar size. Preserve the front-facing retro terminal silhouette, its narrow left pane with mint > prompt and amber underscore cursor, the vertical split, and the three header squares. Use only the dark teal, muted teal, mint, and amber family of Siftpane/Bambustand; remove all cyan/red/pink/rainbow Slack brand cues. Rendering must be clean FLAT pixel art as in image 2: completely uniform solid fills, straight horizontal/vertical edges, large square pixels, regular coarse logical pixel grid. Remove the draft's mottled texture, soft lighting, gradients, shadows, perspective and extra bottom drop-shadow; no smoothing or anti-aliasing. Square opaque near-black teal full-bleed background. Centered icon with generous equal safe margins, no pre-rounded outside app-tile corners, no text, no letters, no company marks, no watermark. One polished standalone project icon, not a sheet or presentation. Make the right-hand bubbles as simple as the left-hand prompt.
```

## Repository hero

[README hero](../assets/branding/hero.png) is a **1774 × 887 PNG** (2:1). It pairs the existing terminal/conversation icon with a lowercase retro wordmark and the project tagline. The README uses this in place of the standalone icon and repeated tagline text.

Generated with the built-in image tool using `assets/branding/icon.png` as the identity/style reference. The standalone square icon remains the Slack app upload; the hero is for repository presentation.
