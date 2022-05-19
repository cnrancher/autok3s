//go:build !prod

package ui

import (
	"embed"
)

var assets embed.FS

const DefaultMode = "dev"
