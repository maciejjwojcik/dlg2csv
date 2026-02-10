//go:build golden

package e2e_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/maciejjwojcik/dlg2csv/internal/csv"
	"github.com/maciejjwojcik/dlg2csv/internal/d"
	"github.com/maciejjwojcik/dlg2csv/internal/tra"
	"github.com/stretchr/testify/require"
)

func TestExport_Golden(t *testing.T) {
	outDir := t.TempDir()

	repoRoot, err := os.Getwd()
	require.NoError(t, err)

	// Export writes to CWD -> isolate in temp dir.
	require.NoError(t, os.Chdir(outDir))
	t.Cleanup(func() { _ = os.Chdir(repoRoot) })

	traDir := filepath.Join(repoRoot, "testdata", "mod", "language", "english")
	dDir := filepath.Join(repoRoot, "testdata", "mod", "dlg", "dialogues_compile")

	traByFile, err := tra.ParseDir(traDir)
	require.NoError(t, err)

	dByFile, err := d.ParseDir(dDir)
	require.NoError(t, err)

	_, err = csv.Export(dByFile, traByFile)
	require.NoError(t, err)

	normalize := func(b []byte) string {
		s := string(b)
		s = strings.TrimPrefix(s, "\uFEFF")
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.TrimRight(s, "\n")
		return s
	}

	// Compare all expected CSV files 1:1 with generated CSV in CWD.
	expectedDir := filepath.Join(repoRoot, "testdata", "expected")
	entries, err := os.ReadDir(expectedDir)
	require.NoError(t, err)

	expectedNames := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		expectedNames = append(expectedNames, e.Name())

		want, err := os.ReadFile(filepath.Join(expectedDir, e.Name()))
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(outDir, e.Name()))
		require.NoError(t, err, "missing generated csv: %s", e.Name())

		require.Equal(t, normalize(want), normalize(got), "csv mismatch: %s", e.Name())
	}

	// Ensure no extra CSV files were generated.
	outEntries, err := os.ReadDir(outDir)
	require.NoError(t, err)

	gotNames := make([]string, 0, len(outEntries))
	for _, e := range outEntries {
		if e.IsDir() {
			continue
		}
		gotNames = append(gotNames, e.Name())
	}

	sort.Strings(expectedNames)
	sort.Strings(gotNames)
	require.Equal(t, expectedNames, gotNames, "generated csv file set mismatch")
}
