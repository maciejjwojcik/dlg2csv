package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/maciejjwojcik/dlg2csv/internal/d"
	"github.com/maciejjwojcik/dlg2csv/internal/tra"
)

type ExportResult struct {
	Sheets map[string][][]string
}

func Export(dialogs d.DByFile, tra tra.TraByFile) (ExportResult, error) {
	dKeys := make([]string, 0, len(dialogs))
	for k := range dialogs {
		dKeys = append(dKeys, k)
	}
	sort.Strings(dKeys)

	// loops over .d files and retreieves values from corresponding .tra
	for _, k := range dKeys {
		used := map[int]struct{}{}
		csvFileName := sanitizeFilename(k) + ".csv"
		fmt.Println("creating:", csvFileName)
		f, err := os.Create(csvFileName)
		if err != nil {
			return ExportResult{}, fmt.Errorf("create %s: %w", csvFileName, err)
		}
		w := csv.NewWriter(f)

		if err := w.Write([]string{
			"npc_name",
			"dialogue_id",
			"state",
			"npc_strref",
			"npc_text_en",
			"pc_strref",
			"pc_text_en",
			"goto",
			"comment",
		}); err != nil {
			f.Close()
			return ExportResult{}, fmt.Errorf("write header %s: %w", csvFileName, err)
		}

		makeEmptyRow := func() []string {
			return []string{"", "", "", "", "", "", "", "", ""}
		}

		formatTraID := func(id *int) string {
			if id == nil {
				return ""
			}
			return fmt.Sprintf("@%d", *id)
		}

		formatComment := func(o d.TextOccurrence) string {
			notes := strings.Join(o.Notes, ", ")
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
			default:
				return ""
			}
		}

		occ := dialogs[k]

		for _, o := range occ {
			if o.TraID != nil {
				used[*o.TraID] = struct{}{}
			}
			row := makeEmptyRow()

			// columns which are always filled
			row[0] = o.SpeakerDlg
			row[1] = o.Dialog
			row[2] = o.State
			row[8] = formatComment(o)

			text := tra[k].GetTextByID(o.TraID)

			switch o.Kind {
			case d.KindNPC:
				row[3] = formatTraID(o.TraID)
				row[4] = text
			case d.KindPC:
				row[5] = formatTraID(o.TraID)
				row[6] = text
				row[7] = formatGoto(o)
			default:
				continue
			}

			if err := w.Write(row); err != nil {
				return ExportResult{}, fmt.Errorf("write row %s: %w", csvFileName, err)
			}
		}

		ids := make([]int, 0, len(tra[k].Texts))
		for id := range tra[k].Texts {
			if _, ok := used[id]; ok {
				continue
			}
			ids = append(ids, id)
		}
		sort.Ints(ids)

		for _, id := range ids {
			row := makeEmptyRow()
			row[1] = k
			row[3] = formatTraID(&id)
			row[4] = tra[k].Texts[id]
			row[8] = "UNUSED IN .D"

			if err := w.Write(row); err != nil {
				return ExportResult{}, fmt.Errorf("write unused row %s: %w", csvFileName, err)
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			f.Close()
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

		if err := w.Write([]string{
			"npc_name",
			"dialogue_id",
			"state",
			"npc_strref",
			"npc_text_en",
			"pc_strref",
			"pc_text_en",
			"goto",
			"comment",
		}); err != nil {
			f.Close()
			return ExportResult{}, fmt.Errorf("write header %s: %w", csvFileName, err)
		}

		t := tra[k]

		ids := make([]int, 0, len(t.Texts))
		for id := range t.Texts {
			ids = append(ids, id)
		}
		sort.Ints(ids)

		for _, id := range ids {
			row := []string{
				"", // npc_name
				k,  // dialogue_id
				"", // state
				fmt.Sprintf("@%d", id),
				t.Texts[id],
				"", "", "", // pc_strref, pc_text_en, goto
				"TRA_ONLY", // comment
			}
			if err := w.Write(row); err != nil {
				f.Close()
				return ExportResult{}, fmt.Errorf("write tra-only row %s: %w", csvFileName, err)
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			f.Close()
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
