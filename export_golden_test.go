package dlg2csv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Export is the future public entry point.
// It does NOT exist yet – this test defines the contract.
func TestExport_Golden_DialogsAndStrings(t *testing.T) {
	t.Skip("export not implemented yet")

	outDir := t.TempDir()

	err := Export(Options{
		RepoPath: "testdata/mod",
		Language: "english",
		OutDir:   outDir,
	})
	require.NoError(t, err)

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

		require.Equal(
			t,
			string(want),
			string(got),
			"dialog csv mismatch: %s",
			name,
		)
	}

	// ---- strings.csv ----
	gotStrings, err := os.ReadFile(filepath.Join(outDir, "strings.csv"))
	require.NoError(t, err)

	wantStrings, err := os.ReadFile(filepath.Join("testdata", "expected", "strings.csv"))
	require.NoError(t, err)

	require.Equal(
		t,
		string(wantStrings),
		string(gotStrings),
		"strings.csv mismatch",
	)
}
