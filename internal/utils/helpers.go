package helpers

import (
	"path/filepath"
	"strings"
)

func BaseKey(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}
