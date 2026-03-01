package csv

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/maciejjwojcik/dlg2csv/internal/d"
	"github.com/maciejjwojcik/dlg2csv/internal/tra"
)

var wantHeader = []string{
	"Name",
	"DialogID",
	"State",
	"NPC strref",
	"Dialog",
	"PC strref",
	"Response from player",
	"Goto",
	"Comment",
	"Male NPC",
	"Male PC",
	"Female NPC",
	"Female PC",
}

func padToHeaderLen(row []string) []string {
	// ensure each row has the same number of columns as header
	for len(row) < len(wantHeader) {
		row = append(row, "")
	}
	return row
}

func TestExport_DialogWithUnusedTra_GeneratesExpectedCSV(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	id100 := 100
	id101 := 101

	toDlg := "AC#TEST"
	toState := "NEXT"

	dialogs := d.DByFile{
		"01 Dialog.d": {
			{
				Kind:       d.KindNPC,
				TraID:      &id100,
				SpeakerDlg: "AC#TEST",
				Dialog:     "AC#TEST",
				State:      "START",
				Notes:      []string{"CHAIN", "Hello there."},
				Condition:  `Global("AC#X","GLOBAL",1)`,
			},
			{
				Kind:       d.KindPC,
				TraID:      &id101,
				SpeakerDlg: "AC#TEST",
				Dialog:     "AC#TEST",
				State:      "START",
				ToType:     "EXTERN",
				ToDlg:      &toDlg,
				ToState:    &toState,
				Notes:      []string{"REPLY"},
			},
		},
	}

	tr := tra.TraByFile{
		"01 Dialog.d": mustMakeTra(t, map[string]string{
			"100": "Hello there.",
			"101": "Goodbye.",
			"999": "I am unused.",
		}),
	}

	_, err = Export(dialogs, tr)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	got := mustReadCSV(t, filepath.Join(tmp, "01_Dialog.d.csv"))

	want := [][]string{
		wantHeader,

		// NPC row
		padToHeaderLen([]string{
			"AC#TEST", "AC#TEST", "START",
			"@100", "Hello there.",
			"", "", "",
			`CHAIN | Global("AC#X","GLOBAL",1)`,
		}),

		// PC row with EXTERN target
		padToHeaderLen([]string{
			"AC#TEST", "AC#TEST", "START",
			"", "",
			"@101", "Goodbye.",
			"EXTERN:AC#TEST:NEXT",
			"REPLY",
		}),

		// UNUSED rows
		padToHeaderLen([]string{
			"", "01 Dialog.d", "",
			"@999", "I am unused.",
			"", "", "",
			"UNUSED IN .D",
		}),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("csv mismatch\nGOT : %#v\nWANT: %#v", got, want)
	}
}

func TestExport_TraOnlyFile_GeneratesExpectedCSV(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	dialogs := d.DByFile{} // no .d for this tra

	tr := tra.TraByFile{
		"02_Quest.tra": mustMakeTra(t, map[string]string{
			"10": "Alpha",
			"2":  "Beta",
		}),
	}

	_, err = Export(dialogs, tr)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	got := mustReadCSV(t, filepath.Join(tmp, "02_Quest.tra.csv"))

	// ids are sorted numerically: 2, 10
	want := [][]string{
		wantHeader,
		padToHeaderLen([]string{
			"", "02_Quest.tra", "",
			"@2", "Beta",
			"", "", "",
			"TRA_ONLY",
		}),
		padToHeaderLen([]string{
			"", "02_Quest.tra", "",
			"@10", "Alpha",
			"", "", "",
			"TRA_ONLY",
		}),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("csv mismatch\nGOT : %#v\nWANT: %#v", got, want)
	}
}

func TestExport_GotoFormatting_EXIT_and_EXTERNFallback(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	id1 := 1
	id2 := 2

	dialogs := d.DByFile{
		"03.d": {
			{
				Kind:       d.KindPC,
				TraID:      &id1,
				SpeakerDlg: "NPC",
				Dialog:     "D",
				State:      "S",
				ToType:     "EXIT",
			},
			{
				Kind:       d.KindPC,
				TraID:      &id2,
				SpeakerDlg: "NPC",
				Dialog:     "D",
				State:      "S",
				ToType:     "EXTERN", // but missing ToDlg/ToState -> fallback "EXTERN"
			},
		},
	}

	tr := tra.TraByFile{
		"03.d": mustMakeTra(t, map[string]string{
			"1": "Bye",
			"2": "Go somewhere",
		}),
	}

	_, err = Export(dialogs, tr)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	got := mustReadCSV(t, filepath.Join(tmp, "03.d.csv"))
	if len(got) < 3 {
		t.Fatalf("expected at least 3 rows (header + 2), got %d", len(got))
	}

	const colGoto = 7

	if got[1][colGoto] != "EXIT" {
		t.Fatalf("expected goto EXIT, got %q", got[1][colGoto])
	}

	if got[2][colGoto] != "EXTERN" {
		t.Fatalf("expected goto EXTERN fallback, got %q", got[2][colGoto])
	}

}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"abc", "abc"},
		{"01 Dialog.d", "01_Dialog.d"},
		{"weird/..\\name:*?", "weird_.._name_"},
		{"a b   c", "a_b_c"},
	}

	for _, tc := range tests {
		if got := sanitizeFilename(tc.in); got != tc.want {
			t.Fatalf("sanitizeFilename(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

// ---- helpers ----

func mustReadCSV(t *testing.T, path string) [][]string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
	}()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv %s: %v", path, err)
	}
	return records
}

func mustMakeTra(t *testing.T, texts map[string]string) tra.Tra {
	t.Helper()
	return tra.NewTra(texts)
}
