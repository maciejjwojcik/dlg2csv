//go:build golden

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExport_Golden(t *testing.T) {
	outDir := t.TempDir()

	err := Export(Options{
		RepoPath: filepath.Join("testdata", "mod"),
		Language: "english",
		OutDir:   outDir,
	})
	require.NoError(t, err)

	normalize := func(b []byte) string {
		s := string(b)
		s = strings.TrimPrefix(s, "\uFEFF")
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.TrimRight(s, "\n")
		return s
	}

	expectedDir := filepath.Join("testdata", "expected")
	entries, err := os.ReadDir(expectedDir)
	require.NoError(t, err)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()

		var gotPath string
		if name == "items.csv" {
			// strings.csv
			gotPath = filepath.Join(outDir, "strings.csv")
		} else {
			// dialog csv
			gotPath = filepath.Join(outDir, "dialogs", name)
		}

		got, err := os.ReadFile(gotPath)
		require.NoError(t, err, "missing generated csv: %s", name)

		want, err := os.ReadFile(filepath.Join(expectedDir, name))
		require.NoError(t, err)

		require.Equal(t, normalize(want), normalize(got), "csv mismatch: %s", name)
	}
}
