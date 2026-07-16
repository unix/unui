package skill

import (
	"embed"
	"io/fs"
)

const (
	Name    = "unui"
	Version = "1"
)

//go:embed unui
var embedded embed.FS

func Bundle() (fs.FS, error) {
	return fs.Sub(embedded, Name)
}
