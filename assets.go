package vanguard

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var WebDist embed.FS

func StaticFS() (fs.FS, error) {
	return fs.Sub(WebDist, "web/dist")
}
