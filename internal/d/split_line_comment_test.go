package d

import (
	"testing"
)

func TestSplitLineComment(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantCode string
		wantCmt  string
	}{
		{
			name:     "no_comment",
			in:       `SAY @100`,
			wantCode: `SAY @100`,
			wantCmt:  ``,
		},
		{
			name:     "trailing_comment",
			in:       `SAY @100 // greeting`,
			wantCode: `SAY @100`,
			wantCmt:  `greeting`,
		},
		{
			name:     "only_comment_line",
			in:       `// only a note`,
			wantCode: ``,
			wantCmt:  `only a note`,
		},
		{
			name:     "comment_inside_tilde_is_not_comment",
			in:       `SAY ~this is not // a comment~`,
			wantCode: `SAY ~this is not // a comment~`,
			wantCmt:  ``,
		},
		{
			name:     "spaces_trimmed",
			in:       `   SAY @100   //  greeting   `,
			wantCode: `SAY @100`,
			wantCmt:  `greeting`,
		},
		{
			name:     "double_slash_in_tilde_then_real_comment",
			in:       `SAY ~a//b~ // real`,
			wantCode: `SAY ~a//b~`,
			wantCmt:  `real`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, cmt := splitLineComment(tt.in)
			if code != tt.wantCode {
				t.Fatalf("code mismatch:\n got: %q\nwant: %q", code, tt.wantCode)
			}
			if cmt != tt.wantCmt {
				t.Fatalf("comment mismatch:\n got: %q\nwant: %q", cmt, tt.wantCmt)
			}
		})
	}
}
