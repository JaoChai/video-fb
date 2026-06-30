package producer

import "embed"

// audioAssetsFS holds the bundled royalty-free SFX and ambient beds. Embedding
// (rather than copying from disk like fonts) means selection + project assembly
// work from any working dir with no path resolution, matching the vendored GSAP.
//
//go:embed assets/audio
var audioAssetsFS embed.FS
