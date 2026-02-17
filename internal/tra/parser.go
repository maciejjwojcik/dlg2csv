package tra

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	helpers "github.com/maciejjwojcik/dlg2csv/internal/utils"
)

type TraByFile map[string]Tra

type Tra struct {
	Texts map[string]string
}

func NewTra(texts map[string]string) Tra {
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

	out := make(map[string]string)

	type mode int
	const (
		modeNormal mode = iota
		modeReadMale
		modeReadMaleQuote
		modeReadMaleTilde
	)

	var (
		m      = modeNormal
		curID  string
		b      strings.Builder
		lineNo int
	)

	flushMale := func() error {
		if _, exists := out[curID]; exists {
			return &ParseError{File: fileName, Line: lineNo, Msg: fmt.Sprintf("duplicate string id @%s in file", curID)}
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
			for i < len(afterAt) {
				ch := afterAt[i]
				if ch == ' ' || ch == '\t' || ch == '=' {
					break
				}
				i++
			}
			if i == 0 {
				return nil, &ParseError{File: fileName, Line: lineNo, Msg: "expected id after @"}
			}
			id := afterAt[:i]

			rest := strings.TrimSpace(afterAt[i:])
			eq := strings.Index(rest, "=")
			if eq < 0 {
				return nil, &ParseError{File: fileName, Line: lineNo, Msg: "expected '=' after id"}
			}
			right := strings.TrimSpace(rest[eq+1:])

			if !strings.HasPrefix(right, "~") && !strings.HasPrefix(right, `"`) {
				return nil, &ParseError{File: fileName, Line: lineNo, Msg: `expected '~' or '"' to start string literal`}
			}

			curID = id

			if strings.HasPrefix(right, "~") {
				right = right[1:]
				if end := strings.IndexByte(right, '~'); end >= 0 {
					b.WriteString(right[:end])
					if err := flushMale(); err != nil {
						return nil, err
					}
					continue
				}
				m = modeReadMaleTilde
				b.WriteString(right)
				b.WriteString("\n")
				continue
			}

			// starts with "
			right = right[1:]
			if end := strings.IndexByte(right, '"'); end >= 0 {
				b.WriteString(right[:end])
				if err := flushMale(); err != nil {
					return nil, err
				}
				continue
			}
			m = modeReadMaleQuote
			b.WriteString(right)
			b.WriteString("\n")
			continue

		case modeReadMaleTilde:
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
		case modeReadMaleQuote:
			if before, _, ok := strings.Cut(line, "\""); ok {
				b.WriteString(before)
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
	if m == modeReadMaleTilde || m == modeReadMaleQuote {
		return nil, &ParseError{File: fileName, Line: lineNo, Msg: fmt.Sprintf("unterminated string literal for @%s", curID)}
	}

	tra := NewTra(out)

	return &tra, nil
}

func (t Tra) GetTextByID(id *int) string {
	if id == nil || t.Texts == nil {
		return ""
	}

	key := fmt.Sprintf("%d", *id)
	if txt, ok := t.Texts[key]; ok {
		return txt
	}
	return fmt.Sprintf("#MISSING(@%s)", key)
}
