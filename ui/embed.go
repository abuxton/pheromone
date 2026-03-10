// Package uiassets provides the embedded web UI assets for the Pheromone
// management interface.
package uiassets

import "embed"

// FS is the embedded filesystem containing the management UI assets.
//
//go:embed index.html
var FS embed.FS
