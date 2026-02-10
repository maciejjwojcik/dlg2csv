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

	helpers "github.com/maciejjwojcik/dlg2csv/internal/utils"
)

var (
	// IF ~cond~ THEN BEGIN STATE
	reBeginState = regexp.MustCompile(`(?i)^\s*IF\s*(~.*?~|~~)\s*THEN\s*BEGIN\s+([A-Za-z0-9_#.\-]+)\s*$`)

	// SAY @123
	reSay = regexp.MustCompile(`(?i)^\s*SAY\s+@(\d+)\s*$`)

	// IF ~cond~ THEN REPLY @123 <rest>
	reReply = regexp.MustCompile(`(?i)^\s*IF\s*(~.*?~|~~)\s*THEN\s*REPLY\s+@(\d+)\s*(.*)$`)

	// Targets inside reply "rest"
	reExtern = regexp.MustCompile(`(?i)\bEXTERN\s+([A-Za-z0-9_#.\-]+)\s+([A-Za-z0-9_#.\-]+)\b`)
	reGoto   = regexp.MustCompile(`(?i)\bGOTO\s+([A-Za-z0-9_#.\-]+)\b`)
	reExit   = regexp.MustCompile(`(?i)\bEXIT\b`)

	// CHAIN header with optional IF~...~THEN
	//
	// Examples matched:
	//   CHAIN AC#FPGND strange_woman
	//   CHAIN IF~Global("X","GLOBAL",1)~THEN AC#FPGND strange_woman
	//   CHAIN IF ~Global("X","GLOBAL",1)~ THEN AC#FPGND strange_woman
	//
	// Capturing groups:
	//   m[1] = dialog (e.g. "AC#FPGND")
	//   m[2] = state  (e.g. "strange_woman")
	reChainHeader = regexp.MustCompile(
		`(?i)^\s*CHAIN\s+(?:IF\s*~.*?~\s*THEN\s+)?([A-Za-z0-9_#.\-]+)\s+([A-Za-z0-9_#.\-]+)\s*$`,
	)

	// NPC line inside CHAIN body: @200
	// Capturing groups: m[1] = tra id
	reChainLine = regexp.MustCompile(`^\s*@(\d+)\s*$`)

	// Interjection with IF:
	//   ==JAHEIJ IF ~InParty("JAHEIRA")~ THEN @201
	// Groups:
	//   m[1] = speaker dialog (JAHEIJ)
	//   m[2] = condition inside ~ ~ (InParty("JAHEIRA"))
	//   m[3] = tra id (201)
	reInterjectIf = regexp.MustCompile(
		`(?i)^\s*==\s*([A-Za-z0-9_#.\-]+)\s+IF\s*~(.*?)~\s*THEN\s+@(\d+)\s*$`,
	)

	// Interjection without IF:
	//   ==AC#WOMAN @204
	// Groups:
	//   m[1] = speaker dialog
	//   m[2] = tra id
	reInterject = regexp.MustCompile(
		`(?i)^\s*==\s*([A-Za-z0-9_#.\-]+)\s+@(\d+)\s*$`,
	)

	// EXTEND_BOTTOM ~PGOND~ 0
	// EXTEND_TOP ~SOMEDLG~ some_state
	// m[1] = dlg, m[2] = state
	reExtend = regexp.MustCompile(
		`(?i)^\s*EXTEND_(?:BOTTOM|TOP)\s+~([^~]+)~\s+([A-Za-z0-9_#.\-]+)\s*$`,
	)

	reBeginDlg = regexp.MustCompile(
		`(?i)^\s*BEGIN\s+~?([A-Za-z0-9_#.\-]+)~?\s*$`,
	)
)

type mode int

const (
	modeNormal mode = iota
	modeChain
	modeState
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
		out[helpers.BaseKey(name)] = m
	}

	return out, nil
}

func ParseFile(path string) ([]TextOccurrence, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			return
		}
	}()

	return ParseReader(f, filepath.Base(path))
}

func ParseReader(r io.Reader, fileName string) ([]TextOccurrence, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		out []TextOccurrence

		currentDialog      string
		currentState       string
		currentSpeaker     string
		replyIndex         int
		inState            bool
		inExtend           bool
		pendingNotes       []string
		stateEntryCond     string
		pendingChainHeader string

		mode = modeNormal
	)
	lastChainTextIdx := -1

	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()

		switch mode {
		case modeNormal:
			line, comment := splitLineComment(raw)
			line = strings.TrimSpace(line)

			// Handle multi-line CHAIN IF ~...~ THEN ... headers.
			// If we previously saw a CHAIN line that didn't close the ~ ~, keep appending lines.
			if pendingChainHeader != "" {
				// preserve a space/newline so tokens don't glue
				pendingChainHeader += " " + strings.TrimSpace(line)
				line = pendingChainHeader

				// still not closed? keep waiting
				if !strings.Contains(strings.ToUpper(line), "~ THEN") && !strings.Contains(line, "~\tTHEN") && !strings.Contains(line, "~  THEN") {
					continue
				}

				// got a complete header; clear buffer and parse normally below
				pendingChainHeader = ""
			} else {
				trim := strings.TrimSpace(line)
				// Start buffering if this looks like a CHAIN IF header that hasn't closed ~...~ yet.
				if strings.HasPrefix(strings.ToUpper(trim), "CHAIN IF") && strings.Contains(trim, "~") && !strings.Contains(strings.ToUpper(trim), "~ THEN") {
					pendingChainHeader = trim
					continue
				}
			}

			// comment-only line
			if line == "" && comment != "" {
				if looksLikeWeiduCode(comment) {
					continue // ignore commented-out code
				}
				if currentDialog != "" {
					pendingNotes = append(pendingNotes, comment)
				}
				continue
			}

			// inline comment (code + comment)
			if comment != "" && currentDialog != "" {
				pendingNotes = append(pendingNotes, comment)
			}

			if line == "" {
				continue
			}

			// BEGIN <dialog>
			if mm := reBeginDlg.FindStringSubmatch(line); mm != nil {
				currentDialog = mm[1]
				currentSpeaker = currentDialog
				currentState = ""
				inState = false
				inExtend = false
				replyIndex = 0
				stateEntryCond = ""

				pendingNotes = nil
				continue
			}

			// EXTEND
			if mm := reExtend.FindStringSubmatch(line); mm != nil {
				dlg := strings.TrimSpace(mm[1])
				st := strings.TrimSpace(mm[2])

				currentDialog = dlg
				currentSpeaker = dlg
				currentState = st
				replyIndex = 0
				inState = true
				inExtend = true
				mode = modeNormal
				continue
			}

			if mm := reChainHeader.FindStringSubmatch(line); mm != nil {
				if currentDialog == "" {
					currentDialog = mm[1]
				}
				currentSpeaker = mm[1]
				currentState = mm[2]
				replyIndex = 0
				inState = true
				mode = modeChain
				lastChainTextIdx = -1
				continue
			}

			// IF ... THEN BEGIN <state>
			if mm := reBeginState.FindStringSubmatch(line); mm != nil {
				if currentDialog == "" {
					return nil, fmt.Errorf("%s:%d: state defined before BEGIN", fileName, lineNo)
				}
				currentState = mm[2]
				currentSpeaker = currentDialog
				stateEntryCond = normalizeCondition(mm[1])
				replyIndex = 0
				inState = true
				mode = modeState
				continue
			}

			// END closes EXTEND_* block only
			if strings.EqualFold(line, "END") {
				if inExtend {
					inExtend = false
					inState = false
					currentState = ""
					currentSpeaker = ""
					currentDialog = ""
				}
				continue
			}

			// SAY @id (NPC line)
			if mm := reSay.FindStringSubmatch(line); mm != nil {
				if currentDialog == "" || currentState == "" || !inState {
					return nil, fmt.Errorf("%s:%d: SAY outside state", fileName, lineNo)
				}
				id, err := strconv.Atoi(mm[1])
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
				})
				pendingNotes = nil
				continue
			}

			// IF ... THEN REPLY @id <rest> (PC line)
			if mm := reReply.FindStringSubmatch(line); mm != nil {
				if currentDialog == "" || currentState == "" || !inState {
					fmt.Printf("DEBUG: mode=%v inState=%v dlg=%q state=%q line=%q\n", mode, inState, currentDialog, currentState, line)
					return nil, fmt.Errorf("%s:%d: REPLY outside state", fileName, lineNo)
				}

				cond := normalizeCondition(strings.TrimSpace(mm[1]))

				id, err := strconv.Atoi(mm[2])
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid TraID in REPLY: %w", fileName, lineNo, err)
				}

				rest := strings.TrimSpace(mm[3])

				occ := TextOccurrence{
					TraID:      intPtr(id),
					Kind:       KindPC,
					SpeakerDlg: "",
					Dialog:     currentDialog,
					State:      currentState,
					ReplyIndex: intPtr(replyIndex),
					Condition:  cond,
					Notes:      pendingNotes,
				}
				pendingNotes = nil
				replyIndex++

				// Target parsing (best-effort)
				if t := reExtern.FindStringSubmatch(rest); t != nil {
					occ.ToType = "EXTERN"
					occ.ToDlg = strPtr(t[1])
					occ.ToState = strPtr(t[2])
				} else if t := reGoto.FindStringSubmatch(rest); t != nil {
					occ.ToType = "GOTO"
					occ.ToDlg = strPtr(currentDialog)
					occ.ToState = strPtr(t[1])
				} else if reExit.MatchString(rest) {
					occ.ToType = "EXIT"
				}

				out = append(out, occ)
				continue
			}

		case modeChain:
			line, comment := splitLineComment(raw)
			line = strings.TrimSpace(line)

			// comment-only line
			if line == "" && comment != "" {
				if looksLikeWeiduCode(comment) {
					continue // ignore commented-out code
				}
				pendingNotes = append(pendingNotes, comment)
				continue
			}

			// inline comment (code + comment)
			if comment != "" {
				pendingNotes = append(pendingNotes, comment)
			}

			if line == "" {
				continue
			}

			// END ends CHAIN body; after END come REPLY lines in modeNormal for the same state
			if strings.EqualFold(line, "END") {
				mode = modeNormal
				inState = true
				lastChainTextIdx = -1
				continue
			}

			// Auto-transition in CHAIN body: EXTERN ... or EXIT
			// Attach to the last emitted NPC text occurrence in this CHAIN.
			if mm := reExtern.FindStringSubmatch(line); mm != nil {
				if lastChainTextIdx < 0 {
					return nil, fmt.Errorf("%s:%d: EXTERN in CHAIN body without preceding text", fileName, lineNo)
				}
				out[lastChainTextIdx].ToType = "EXTERN"
				out[lastChainTextIdx].ToDlg = strPtr(mm[1])
				out[lastChainTextIdx].ToState = strPtr(mm[2])

				// CHAIN bodies can end without END (common pattern)
				mode = modeNormal
				inState = true
				lastChainTextIdx = -1
				continue
			}

			if reExit.MatchString(line) {
				if lastChainTextIdx < 0 {
					return nil, fmt.Errorf("%s:%d: EXIT in CHAIN body without preceding text", fileName, lineNo)
				}
				out[lastChainTextIdx].ToType = "EXIT"
				out[lastChainTextIdx].ToDlg = nil
				out[lastChainTextIdx].ToState = nil

				// CHAIN bodies can end without END (common pattern)
				mode = modeNormal
				inState = true
				lastChainTextIdx = -1
				continue
			}

			// Interjection with IF:
			// ==JAHEIJ IF ~InParty("JAHEIRA")~ THEN @201
			if mm := reInterjectIf.FindStringSubmatch(line); mm != nil {
				if currentDialog == "" || currentState == "" {
					return nil, fmt.Errorf("%s:%d: interjection outside dialog/state", fileName, lineNo)
				}
				speaker := mm[1]
				cond := strings.TrimSpace(mm[2])

				id, err := strconv.Atoi(mm[3])
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid TraID in interjection IF: %w", fileName, lineNo, err)
				}

				out = append(out, TextOccurrence{
					TraID:      intPtr(id),
					Kind:       KindNPC,
					SpeakerDlg: speaker,
					Dialog:     currentDialog,
					State:      currentState,
					Condition:  cond,
					Notes:      pendingNotes,
				})
				pendingNotes = nil
				lastChainTextIdx = len(out) - 1
				continue
			}

			// Interjection without IF:
			// ==AC#WOMAN @204
			if mm := reInterject.FindStringSubmatch(line); mm != nil {
				if currentDialog == "" || currentState == "" {
					return nil, fmt.Errorf("%s:%d: interjection outside dialog/state", fileName, lineNo)
				}
				speaker := mm[1]
				id, err := strconv.Atoi(mm[2])
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid TraID in interjection: %w", fileName, lineNo, err)
				}

				out = append(out, TextOccurrence{
					TraID:      intPtr(id),
					Kind:       KindNPC,
					SpeakerDlg: speaker,
					Dialog:     currentDialog,
					State:      currentState,
					Notes:      pendingNotes,
				})
				pendingNotes = nil
				lastChainTextIdx = len(out) - 1
				continue
			}

			// Normal NPC line inside CHAIN body: @200
			if mm := reChainLine.FindStringSubmatch(line); mm != nil {
				if currentDialog == "" || currentState == "" || currentSpeaker == "" {
					return nil, fmt.Errorf("%s:%d: chain line outside dialog/state", fileName, lineNo)
				}
				id, err := strconv.Atoi(mm[1])
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid TraID in chain line: %w", fileName, lineNo, err)
				}

				out = append(out, TextOccurrence{
					TraID:      intPtr(id),
					Kind:       KindNPC,
					SpeakerDlg: currentSpeaker,
					Dialog:     currentDialog,
					State:      currentState,
					Notes:      pendingNotes,
				})
				pendingNotes = nil
				lastChainTextIdx = len(out) - 1
				continue
			}

			if reExit.MatchString(line) {
				if lastChainTextIdx < 0 {
					return nil, fmt.Errorf("%s:%d: EXIT in CHAIN body without preceding text", fileName, lineNo)
				}
				out[lastChainTextIdx].ToType = "EXIT"
				out[lastChainTextIdx].ToDlg = nil
				out[lastChainTextIdx].ToState = nil

				// IMPORTANT: CHAIN body may end here (no END)
				mode = modeNormal
				inState = true
				lastChainTextIdx = -1
				continue
			}

			// Ignore other lines in CHAIN body
			continue
		case modeState:
			line, comment := splitLineComment(raw)
			line = strings.TrimSpace(line)

			// comment-only line
			if line == "" && comment != "" {
				if looksLikeWeiduCode(comment) {
					continue
				}
				pendingNotes = append(pendingNotes, comment)
				continue
			}

			// inline comment (code + comment)
			if comment != "" {
				pendingNotes = append(pendingNotes, comment)
			}

			if line == "" {
				continue
			}

			// END closes this state block
			if strings.EqualFold(line, "END") {
				inState = false
				currentState = ""
				currentSpeaker = ""
				mode = modeNormal
				continue
			}

			// SAY @id
			if mm := reSay.FindStringSubmatch(line); mm != nil {
				id, err := strconv.Atoi(mm[1])
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid TraID in SAY: %w", fileName, lineNo, err)
				}

				out = append(out, TextOccurrence{
					TraID:      intPtr(id),
					Kind:       KindNPC,
					SpeakerDlg: currentSpeaker,
					Dialog:     currentDialog,
					State:      currentState,
					Condition:  stateEntryCond,
					Notes:      pendingNotes,
				})
				pendingNotes = nil
				lastChainTextIdx = len(out) - 1
				continue
			}

			// "=" line → ignore (continuation marker)
			if line == "=" {
				continue
			}

			// "@id" continuation line
			if mm := reChainLine.FindStringSubmatch(line); mm != nil {
				id, err := strconv.Atoi(mm[1])
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid TraID in continuation: %w", fileName, lineNo, err)
				}

				out = append(out, TextOccurrence{
					TraID:      intPtr(id),
					Kind:       KindNPC,
					SpeakerDlg: currentSpeaker,
					Dialog:     currentDialog,
					State:      currentState,
					Condition:  stateEntryCond,
					Notes:      pendingNotes,
				})
				pendingNotes = nil
				lastChainTextIdx = len(out) - 1
				continue
			}

			// IF ... THEN REPLY @id <rest>
			if mm := reReply.FindStringSubmatch(line); mm != nil {
				cond := normalizeCondition(mm[1])

				id, err := strconv.Atoi(mm[2])
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid TraID in REPLY: %w", fileName, lineNo, err)
				}

				rest := strings.TrimSpace(mm[3])

				occ := TextOccurrence{
					TraID:      intPtr(id),
					Kind:       KindPC,
					Dialog:     currentDialog,
					State:      currentState,
					ReplyIndex: intPtr(replyIndex),
					Condition:  cond,
					Notes:      pendingNotes,
				}
				pendingNotes = nil
				replyIndex++

				if t := reGoto.FindStringSubmatch(rest); t != nil {
					occ.ToType = "GOTO"
					occ.ToDlg = strPtr(currentDialog)
					occ.ToState = strPtr(t[1])
				} else if reExit.MatchString(rest) {
					occ.ToType = "EXIT"
				}

				out = append(out, occ)
				continue
			}

			// IF ~~ THEN DO ~...~ EXIT
			if strings.Contains(line, "DO") && reExit.MatchString(line) {
				// traktuj jako EXIT bez tekstu
				continue
			}

			// ignore everything else
			continue
		}

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

func looksLikeWeiduCode(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	if strings.HasPrefix(s, "==") {
		return true
	}
	if strings.HasPrefix(s, "@") {
		return true
	}

	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	tok := strings.ToUpper(fields[0])

	switch tok {
	case "IF", "CHAIN", "BEGIN", "SAY", "DO", "EXTERN", "EXIT":
		return true
	default:
		return false
	}
}
