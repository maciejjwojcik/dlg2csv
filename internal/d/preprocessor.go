package d

import "strings"

// SplitComment strips comments from raw line and returns (code, comment).
// comment is concatenated text found in // and /* */ outside ~...~.
type CommentSplitter struct {
	inBlockComment bool
}

func (s *CommentSplitter) Split(raw string) (code string, comment string) {
	if raw == "" {
		return "", ""
	}

	inTilde := false // <-- reset per line

	out := make([]byte, 0, len(raw))
	cmt := make([]byte, 0, len(raw))

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if s.inBlockComment {
			if ch == '*' && i+1 < len(raw) && raw[i+1] == '/' {
				s.inBlockComment = false
				i++
				if len(cmt) > 0 && cmt[len(cmt)-1] != ' ' {
					cmt = append(cmt, ' ')
				}
				continue
			}
			cmt = append(cmt, ch)
			continue
		}

		if ch == '~' {
			inTilde = !inTilde
			out = append(out, ch)
			continue
		}

		if !inTilde && ch == '/' && i+1 < len(raw) && raw[i+1] == '/' {
			// line comment
			cmt = append(cmt, raw[i+2:]...)
			break
		}

		if !inTilde && ch == '/' && i+1 < len(raw) && raw[i+1] == '*' {
			// block comment start
			s.inBlockComment = true
			i++
			continue
		}

		out = append(out, ch)
	}

	return strings.TrimSpace(string(out)), strings.TrimSpace(string(cmt))
}
