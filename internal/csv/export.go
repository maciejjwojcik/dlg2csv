package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
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
	keys := make([]string, 0, len(dialogs))
	for k := range dialogs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
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
		// base := keyBase(k)

		for _, o := range occ {
			row := makeEmptyRow()

			// columns which are always filled
			row[0] = o.SpeakerDlg
			row[1] = o.Dialog
			row[2] = o.State
			row[8] = formatComment(o)

			switch o.Kind {
			case d.KindNPC:
				row[3] = formatTraID(o.TraID)
				row[4] = "text" // placeholder
			case d.KindPC:
				row[5] = formatTraID(o.TraID)
				row[6] = "text" // placeholder
				row[7] = formatGoto(o)
			default:
				continue
			}

			if err := w.Write(row); err != nil {
				return ExportResult{}, fmt.Errorf("write row %s: %w", csvFileName, err)
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

func keyBase(name string) string {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}
