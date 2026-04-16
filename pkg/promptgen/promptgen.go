package promptgen

import (
	"strings"
)

// image extensions handled as "Прочитай изображение ..."
var imageExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
	".svg":  true,
}

// IsImage returns true for jpg/png/gif/webp/bmp/svg by extension
func IsImage(path string) bool {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return imageExt[strings.ToLower(path[i:])]
	}
	return false
}

// Line returns a single prompt line for one path — just the path itself
func Line(path string) string {
	return path
}

// Build returns newline-joined prompt for a list of absolute paths
func Build(paths []string) string {
	b := strings.Builder{}
	for i, p := range paths {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(Line(p))
	}
	return b.String()
}
