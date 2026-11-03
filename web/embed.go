package web

import "embed"

// Dist contains the production Vite build. Run `npm run build` in frontend
// before compiling the Go executable.
//
//go:embed dist
var Dist embed.FS
