//go:build prod
// +build prod

package ui

import (
	"embed"
)

//go:embed static
var assets embed.FS

const DefaultMode = "prod"
