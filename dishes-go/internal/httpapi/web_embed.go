package httpapi

import (
	"embed"
	"io/fs"
)

//go:embed web/dist
var webDist embed.FS

func EmbeddedWebDist() fs.FS {
	sub, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		return webDist
	}
	return sub
}

