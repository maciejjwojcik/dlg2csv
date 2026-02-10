//go:build golden

package e2e_test

import (
	"os"
	"path/filepath"
	"runtime"
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

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	// internal/e2e -> internal -> repo root
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	// Export writes to CWD -> isolate in temp dir.
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(outDir))
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

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
