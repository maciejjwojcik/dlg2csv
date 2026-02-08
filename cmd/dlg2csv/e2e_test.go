//go:build golden

package dlg2csv_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExport_Golden_DialogsAndStrings(t *testing.T) {
	outDir := t.TempDir()

	err := Export(Options{
		RepoPath: filepath.Join("testdata", "mod"),
		Language: "english",
		OutDir:   outDir,
	})
	require.NoError(t, err)

	normalize := func(b []byte) string {
		s := string(b)
		return strings.ReplaceAll(s, "\r\n", "\n")
	}

	// ---- dialogs/*.csv ----
	expectedDialogsDir := filepath.Join("testdata", "expected", "dialogs")
	expectedEntries, err := os.ReadDir(expectedDialogsDir)
	require.NoError(t, err)

	for _, e := range expectedEntries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		gotPath := filepath.Join(outDir, "dialogs", name)
		wantPath := filepath.Join(expectedDialogsDir, name)

		got, err := os.ReadFile(gotPath)
		require.NoError(t, err, "missing generated dialog csv: %s", name)

		want, err := os.ReadFile(wantPath)
		require.NoError(t, err)

		require.Equal(t, normalize(want), normalize(got), "dialog csv mismatch: %s", name)
	}

	// ---- strings.csv ----
	gotStrings, err := os.ReadFile(filepath.Join(outDir, "strings.csv"))
	require.NoError(t, err)

	wantStrings, err := os.ReadFile(filepath.Join("testdata", "expected", "strings.csv"))
	require.NoError(t, err)

	require.Equal(t, normalize(wantStrings), normalize(gotStrings), "strings.csv mismatch")
}
