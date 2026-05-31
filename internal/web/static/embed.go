package static

import "embed"

//go:embed tailwind.js alpine.min.js
var Assets embed.FS
