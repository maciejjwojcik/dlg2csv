package d

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reBeginDialog = regexp.MustCompile(`(?i)^\s*BEGIN\s+([A-Za-z0-9_#\.\-]+)\s*$`)

	// IF ~cond~ THEN BEGIN STATE
	reBeginState = regexp.MustCompile(`(?i)^\s*IF\s+(~.*?~|~~)\s+THEN\s+BEGIN\s+([A-Za-z0-9_#\.\-]+)\s*$`)

	// SAY @123
	reSay = regexp.MustCompile(`(?i)^\s*SAY\s+@(\d+)\s*$`)

	// IF ~cond~ THEN REPLY @123 <rest>
	reReply = regexp.MustCompile(`(?i)^\s*IF\s+(~.*?~|~~)\s+THEN\s+REPLY\s+@(\d+)\s*(.*)$`)

	// Targets inside reply "rest"
	reExtern = regexp.MustCompile(`(?i)\bEXTERN\s+([A-Za-z0-9_#\.\-]+)\s+([A-Za-z0-9_#\.\-]+)\b`)
	reGoto   = regexp.MustCompile(`(?i)\bGOTO\s+([A-Za-z0-9_#\.\-]+)\b`)
	reExit   = regexp.MustCompile(`(?i)\bEXIT\b`)
)

type TextKind string

const (
	KindNPC TextKind = "NPC"
	KindPC  TextKind = "PC"
)

type DByFile map[string][]TextOccurrence

type TextOccurrence struct {
	TraID *int

	Kind       TextKind
	SpeakerDlg string

	Dialog string
	State  string

	ReplyIndex *int

	ToType  string
	ToDlg   *string
	ToState *string

	Condition string // IF ~...~

	Notes []string
}

func ParseDir(dir string) (DByFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.HasSuffix(strings.ToLower(name), ".d") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	out := make(DByFile, len(files))
	for _, name := range files {
		full := filepath.Join(dir, name)
		m, err := ParseFile(full)
		if err != nil {
			return nil, err
		}
		out[name] = m
	}

	return out, nil
}

func ParseFile(path string) ([]TextOccurrence, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ParseReader(f, filepath.Base(path))
}

func ParseReader(r io.Reader, fileName string) ([]TextOccurrence, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		out []TextOccurrence

		currentDialog  string
		currentState   string
		currentSpeaker string
		replyIndex     int
		inState        bool
		pendingNotes   []string
	)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		line, comment := splitLineComment(raw)
		if comment != "" {
			pendingNotes = append(pendingNotes, comment)
		}

		if line == "" {
			continue
		}

		// BEGIN <dialog>
		if m := reBeginDialog.FindStringSubmatch(line); m != nil {
			currentDialog = m[1]
			// Reasonable defaults
			currentSpeaker = currentDialog
			// state is not set here
			continue
		}

		// IF ... THEN BEGIN <state>
		if m := reBeginState.FindStringSubmatch(line); m != nil {
			if currentDialog == "" {
				return nil, fmt.Errorf("%s:%d: state defined before BEGIN", fileName, lineNo)
			}
			currentState = m[2]
			currentSpeaker = currentDialog // NPC speaking dialog
			replyIndex = 0
			inState = true
			// We don’t emit anything for state header
			continue
		}

		// END closes state (we only care for replyIndex reset which already happens on new state)
		if strings.EqualFold(line, "END") {
			inState = false
			continue
		}

		// SAY @id (NPC line)
		if m := reSay.FindStringSubmatch(line); m != nil {
			if currentDialog == "" || currentState == "" || !inState {
				// In practice some files might be weird; treat as error for now (easier debugging)
				return nil, fmt.Errorf("%s:%d: SAY outside state", fileName, lineNo)
			}
			id, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, fmt.Errorf("%s:%d: invalid TraID in SAY: %w", fileName, lineNo, err)
			}
			out = append(out, TextOccurrence{
				TraID:      intPtr(id),
				Kind:       KindNPC,
				SpeakerDlg: currentSpeaker,
				Dialog:     currentDialog,
				State:      currentState,
				Notes:      pendingNotes,
				// ReplyIndex nil for NPC
			})
			pendingNotes = nil
			continue
		}

		// IF ... THEN REPLY @id <rest> (PC line)
		if m := reReply.FindStringSubmatch(line); m != nil {
			if currentDialog == "" || currentState == "" || !inState {
				return nil, fmt.Errorf("%s:%d: REPLY outside state", fileName, lineNo)
			}

			condRaw := strings.TrimSpace(m[1])
			cond := normalizeCondition(condRaw)

			id, err := strconv.Atoi(m[2])
			if err != nil {
				return nil, fmt.Errorf("%s:%d: invalid TraID in REPLY: %w", fileName, lineNo, err)
			}

			rest := strings.TrimSpace(m[3])

			occ := TextOccurrence{
				TraID:      intPtr(id),
				Kind:       KindPC,
				SpeakerDlg: "", // PC
				Dialog:     currentDialog,
				State:      currentState,
				ReplyIndex: intPtr(replyIndex),
				Condition:  cond,
				Notes:      pendingNotes,
			}
			pendingNotes = nil
			replyIndex++

			// Target parsing (best-effort)
			if mm := reExtern.FindStringSubmatch(rest); mm != nil {
				occ.ToType = "EXTERN"
				occ.ToDlg = strPtr(mm[1])
				occ.ToState = strPtr(mm[2])
			} else if mm := reGoto.FindStringSubmatch(rest); mm != nil {
				occ.ToType = "GOTO"
				occ.ToDlg = strPtr(currentDialog)
				occ.ToState = strPtr(mm[1])
			} else if reExit.MatchString(rest) {
				occ.ToType = "EXIT"
			} else {
				occ.ToType = "" // unknown/none
			}

			out = append(out, occ)
			continue
		}

		// Otherwise ignore unknown lines for now.
		// If you prefer strict mode, return an error here with fileName:lineNo.
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: scan error: %w", fileName, err)
	}
	return out, nil
}

// splitLineComment returns code and trailing // comment.
// Returned code is already strings.TrimSpace'd.
func splitLineComment(line string) (code string, comment string) {
	inTilde := false

	for i := 0; i < len(line)-1; i++ {
		ch := line[i]

		// toggle ~...~
		if ch == '~' {
			inTilde = !inTilde
			continue
		}

		// start of // comment (only if not inside ~...~)
		if !inTilde && ch == '/' && line[i+1] == '/' {
			code = strings.TrimSpace(line[:i])
			comment = strings.TrimSpace(line[i+2:])
			return
		}
	}

	// no comment
	code = strings.TrimSpace(line)
	comment = ""
	return
}

func normalizeCondition(cond string) string {
	cond = strings.TrimSpace(cond)
	// cond is "~...~" or "~~"
	if cond == "~~" {
		return ""
	}
	// strip outer ~ ~ if present
	if strings.HasPrefix(cond, "~") && strings.HasSuffix(cond, "~") && len(cond) >= 2 {
		return strings.TrimSpace(cond[1 : len(cond)-1])
	}
	return cond
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
