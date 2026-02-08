package tra

import (
	"strings"
	"testing"
)

func TestParseReader(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      map[int]string
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "single line",
			input: "@100 = ~Hello there, stranger.~\n",
			want:  map[int]string{100: "Hello there, stranger."},
		},
		{
			name:  "leading/trailing whitespace and comments",
			input: "   // comment\n\n   @101   =   ~Hi~   \n",
			want:  map[int]string{101: "Hi"},
		},
		{
			name:  "female on same line is ignored but male kept",
			input: "@102 = ~Hello male.~ ~Hello female.~\n",
			want:  map[int]string{102: "Hello male."},
		},
		{
			name: "multiline male",
			input: "@200 = ~Line1\n" +
				"Line2\n" +
				"Line3~\n",
			want: map[int]string{200: "Line1\nLine2\nLine3"},
		},
		{
			name: "multiline male containing @ at line start should not start new entry",
			input: "@300 = ~Line1\n" +
				"@NOT_AN_ENTRY just text\n" +
				"Line3~\n" +
				"@301 = ~Next~\n",
			want: map[int]string{
				300: "Line1\n@NOT_AN_ENTRY just text\nLine3",
				301: "Next",
			},
		},
		{
			name:      "duplicate id in same file returns error",
			input:     "@400 = ~A~\n@400 = ~B~\n",
			wantErr:   true,
			errSubstr: "duplicate",
		},
		{
			name:      "unterminated multiline male returns error",
			input:     "@500 = ~Line1\nLine2\n",
			wantErr:   true,
			errSubstr: "unterminated",
		},
		{
			name:      "missing opening tilde returns error",
			input:     "@600 = Hello\n",
			wantErr:   true,
			errSubstr: "expected '~'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReader(strings.NewReader(tt.input), "test.tra")

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error to contain %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("map size mismatch: got %d, want %d; got=%v", len(got), len(tt.want), got)
			}
			for id, wantStr := range tt.want {
				gotStr, ok := got[id]
				if !ok {
					t.Fatalf("missing id %d; got=%v", id, got)
				}
				if gotStr != wantStr {
					t.Fatalf("id %d mismatch:\nGOT:\n%q\nWANT:\n%q", id, gotStr, wantStr)
				}
			}
		})
	}
}
