package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/maciejjwojcik/dlg2csv/internal/d"
	"github.com/maciejjwojcik/dlg2csv/internal/tra"
)

type ExportResult struct {
	Sheets map[string][][]string
}

var header = []string{
	"Name",     // 0
	"DialogID", // 1
	"State",    // 2

	"NPC strref", // 3
	"Dialog",     // 4

	"PC strref",            // 5
	"Response from player", // 6
	"Goto",                 // 7

	"Comment", // 8

	"Male NPC",   // 9
	"Male PC",    // 10
	"Female NPC", // 11
	"Female PC",  // 12
}

const (
	colName     = 0
	colDialogID = 1
	colState    = 2

	colNPCStrref = 3
	colNPCText   = 4

	colPCStrref = 5
	colPCText   = 6
	colGoto     = 7

	colComment = 8

	// translator-only columns (must remain empty in export)
	/* colMaleNPC   = 9
	colMalePC    = 10
	colFemaleNPC = 11
	colFemalePC  = 12 */
)

func Export(dialogs d.DByFile, tra tra.TraByFile) (ExportResult, error) {
	dKeys := make([]string, 0, len(dialogs))
	for k := range dialogs {
		dKeys = append(dKeys, k)
	}
	sort.Strings(dKeys)

	makeEmptyRow := func() []string {
		return make([]string, len(header))
	}
	sortTraIDs := func(ids []string) {
		sort.Slice(ids, func(i, j int) bool {
			ai, aErr := strconv.Atoi(ids[i])
			aj, bErr := strconv.Atoi(ids[j])
			if aErr == nil && bErr == nil {
				return ai < aj
			}
			if aErr == nil {
				return true
			}
			if bErr == nil {
				return false
			}
			return ids[i] < ids[j]
		})
	}

	// loops over .d files and retrieves values from corresponding .tra
	for _, k := range dKeys {
		used := map[string]struct{}{}
		csvFileName := sanitizeFilename(k) + ".csv"
		fmt.Println("creating:", csvFileName)
		f, err := os.Create(csvFileName)
		if err != nil {
			return ExportResult{}, fmt.Errorf("create %s: %w", csvFileName, err)
		}
		w := csv.NewWriter(f)

		if err := w.Write(header); err != nil {
			return ExportResult{}, fmt.Errorf("write header %s: %w", csvFileName, err)
		}

		formatTraID := func(id *int) string {
			if id == nil {
				return ""
			}
			return fmt.Sprintf("@%d", *id)
		}

		formatComment := func(o d.TextOccurrence, text string) string {
			var filtered []string

			for _, note := range o.Notes {
				if !isDuplicateComment(note, text) {
					filtered = append(filtered, note)
				}
			}

			notes := strings.Join(filtered, ", ")

			if o.Condition == "" {
				return notes
			}
			if notes == "" {
				return o.Condition
			}
			return notes + " | " + o.Condition
		}

		formatGoto := func(o d.TextOccurrence) string {
			switch strings.ToUpper(o.ToType) {
			case "EXIT":
				return "EXIT"
			case "EXTERN":
				if o.ToDlg != nil && o.ToState != nil {
					return fmt.Sprintf("EXTERN:%s:%s", *o.ToDlg, *o.ToState)
				}
				return "EXTERN" // fallback
			case "GOTO":
				if o.ToState != nil {
					return *o.ToState
				}
				return "GOTO" // fallback
			default:
				return ""
			}
		}
		occ := dialogs[k]

		for _, o := range occ {
			if o.TraID != nil {
				used[strconv.Itoa(*o.TraID)] = struct{}{}
			}
			row := makeEmptyRow()

			text := tra[k].GetTextByID(o.TraID)

			// columns always filled
			row[colName] = o.SpeakerDlg
			row[colDialogID] = o.Dialog
			row[colState] = o.State
			row[colComment] = formatComment(o, text)

			switch o.Kind {
			case d.KindNPC:
				row[colNPCStrref] = formatTraID(o.TraID)
				row[colNPCText] = text

			case d.KindPC:
				row[colPCStrref] = formatTraID(o.TraID)
				row[colPCText] = text
				row[colGoto] = formatGoto(o)

			default:
				continue
			}

			// translator columns intentionally left empty:
			// colMaleNPC, colMalePC, colFemaleNPC, colFemalePC

			if err := w.Write(row); err != nil {
				return ExportResult{}, fmt.Errorf("write row %s: %w", csvFileName, err)
			}
		}

		ids := make([]string, 0, len(tra[k].Texts))
		for id := range tra[k].Texts {
			if _, ok := used[id]; ok {
				continue
			}
			ids = append(ids, id)
		}
		sortTraIDs(ids)

		for _, id := range ids {
			row := makeEmptyRow()

			row[colDialogID] = k
			row[colNPCStrref] = "@" + id
			row[colNPCText] = tra[k].Texts[id]
			row[colComment] = "UNUSED IN .D"

			if err := w.Write(row); err != nil {
				return ExportResult{}, fmt.Errorf("write unused row %s: %w", csvFileName, err)
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			_ = f.Close()
			return ExportResult{}, fmt.Errorf("flush %s: %w", csvFileName, err)
		}

		if err := f.Close(); err != nil {
			return ExportResult{}, fmt.Errorf("close %s: %w", csvFileName, err)
		}

	}

	// loops over .tra files which don't have a corresponding .d, exports as flat csv
	traKeys := make([]string, 0, len(tra))
	for k := range tra {
		traKeys = append(traKeys, k)
	}
	sort.Strings(traKeys)

	for _, k := range traKeys {
		if _, hasDialog := dialogs[k]; hasDialog {
			continue
		}

		csvFileName := sanitizeFilename(k) + ".csv"
		fmt.Println("creating (tra-only):", csvFileName)

		f, err := os.Create(csvFileName)
		if err != nil {
			return ExportResult{}, fmt.Errorf("create %s: %w", csvFileName, err)
		}
		w := csv.NewWriter(f)

		if err := w.Write(header); err != nil {
			return ExportResult{}, fmt.Errorf("write header %s: %w", csvFileName, err)
		}

		t := tra[k]

		ids := make([]string, 0, len(t.Texts))
		for id := range t.Texts {
			ids = append(ids, id)
		}
		sortTraIDs(ids)

		for _, id := range ids {
			row := makeEmptyRow()

			row[colDialogID] = k
			row[colNPCStrref] = "@" + id
			row[colNPCText] = t.Texts[id]
			row[colComment] = "TRA_ONLY"

			if err := w.Write(row); err != nil {
				if err := f.Close(); err != nil {
					return ExportResult{}, fmt.Errorf("close %s: %w", csvFileName, err)
				}
				return ExportResult{}, fmt.Errorf("write tra-only row %s: %w", csvFileName, err)
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			if err := f.Close(); err != nil {
				return ExportResult{}, fmt.Errorf("close %s: %w", csvFileName, err)
			}
			return ExportResult{}, fmt.Errorf("flush %s: %w", csvFileName, err)
		}
		if err := f.Close(); err != nil {
			return ExportResult{}, fmt.Errorf("close %s: %w", csvFileName, err)
		}
	}

	return ExportResult{}, nil
}

func sanitizeFilename(s string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	return re.ReplaceAllString(s, "_")
}

func isDuplicateComment(comment, english string) bool {
	nc := normalizeForCompare(comment)
	ne := normalizeForCompare(english)

	if nc == "" || ne == "" {
		return false
	}

	if nc == ne {
		return true
	}

	if strings.Contains(nc, ne) || strings.Contains(ne, nc) {
		return true
	}

	return false
}

func normalizeForCompare(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")

	replacer := strings.NewReplacer(
		"“", `"`,
		"”", `"`,
		"’", `'`,
		"\"", "",
		".", "",
		",", "",
		"!", "",
		"?", "",
	)
	s = replacer.Replace(s)

	return s
}
