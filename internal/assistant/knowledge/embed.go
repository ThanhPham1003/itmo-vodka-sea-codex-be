package knowledge

import "embed"

// Articles are curated and shipped in the binary; never scan arbitrary repo files.
//
//go:embed articles/*.md
var Files embed.FS
