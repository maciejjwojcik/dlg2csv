package tra

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	helpers "github.com/maciejjwojcik/dlg2csv/internal/utils"
)

type TraByFile map[string]Tra

type Tra struct {
	Texts map[int]string
}

func NewTra(texts map[int]string) Tra {
	return Tra{Texts: texts}
}

type ParseError struct {
	File string
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

func ParseDir(dir string) (TraByFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := strings.ToLower(ent.Name())
		if strings.HasSuffix(strings.ToLower(name), ".tra") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	out := make(TraByFile, len(files))
	for _, name := range files {
		full := filepath.Join(dir, name)
		tra, err := ParseFile(full)
		if err != nil {
			return nil, err
		}
		out[helpers.BaseKey(name)] = *tra
	}

	return out, nil
}

func ParseFile(path string) (*Tra, error) {
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

func ParseReader(r io.Reader, fileName string) (*Tra, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	out := make(map[int]string)

	type mode int
	const (
		modeNormal mode = iota
		modeReadMale
	)

	var (
		m      = modeNormal
		curID  int
		b      strings.Builder
		lineNo int
	)

	flushMale := func() error {
		if _, exists := out[curID]; exists {
			return &ParseError{File: fileName, Line: lineNo, Msg: fmt.Sprintf("duplicate string id @%d in file", curID)}
		}
		out[curID] = b.String()
		b.Reset()
		return nil
	}

	for sc.Scan() {
		lineNo++
		line := sc.Text()

		switch m {
		case modeNormal:
			trim := strings.TrimSpace(line)
			if trim == "" || strings.HasPrefix(trim, "//") {
				continue
			}
			if !strings.HasPrefix(trim, "@") {
				continue
			}

			afterAt := trim[1:]
			i := 0
			for i < len(afterAt) && afterAt[i] >= '0' && afterAt[i] <= '9' {
				i++
			}
			if i == 0 {
				return nil, &ParseError{File: fileName, Line: lineNo, Msg: "expected numeric id after @"}
			}
			id, err := strconv.Atoi(afterAt[:i])
			if err != nil {
				return nil, &ParseError{File: fileName, Line: lineNo, Msg: "invalid id number"}
			}

			rest := strings.TrimSpace(afterAt[i:])
			eq := strings.Index(rest, "=")
			if eq < 0 {
				return nil, &ParseError{File: fileName, Line: lineNo, Msg: "expected '=' after id"}
			}
			right := strings.TrimSpace(rest[eq+1:])

			if !strings.HasPrefix(right, "~") {
				return nil, &ParseError{File: fileName, Line: lineNo, Msg: "expected '~' to start string literal"}
			}
			right = right[1:] // right represents string contents after the opening ~

			curID = id

			if end := strings.IndexByte(right, '~'); end >= 0 { // closing '~' for the first (male) literal on this line
				b.WriteString(right[:end])
				if err := flushMale(); err != nil { // store parsed male string (and detect duplicate IDs)
					return nil, err
				}
				continue
			}

			// multiline male
			m = modeReadMale
			b.WriteString(right)
			b.WriteString("\n")
			continue

		case modeReadMale:
			if end := strings.IndexByte(line, '~'); end >= 0 {
				b.WriteString(line[:end])
				if err := flushMale(); err != nil {
					return nil, err
				}
				m = modeNormal
				continue
			}

			b.WriteString(line)
			b.WriteString("\n")
			continue
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}
	if m == modeReadMale {
		return nil, &ParseError{File: fileName, Line: lineNo, Msg: fmt.Sprintf("unterminated string literal for @%d", curID)}
	}

	tra := NewTra(out)

	return &tra, nil
}

func (t Tra) GetTextByID(id *int) string {
	if id == nil || t.Texts == nil {
		return ""
	}

	if txt, ok := t.Texts[*id]; ok {
		return txt
	}
	return fmt.Sprintf("#MISSING(@%d)", *id)
}
