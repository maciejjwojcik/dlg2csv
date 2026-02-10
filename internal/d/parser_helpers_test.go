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

func TestNormalizeCondition(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"spaces_only", "   \t  ", ""},
		{"double_tilde", "~~", ""},
		{"double_tilde_with_spaces", "  ~~  ", ""},
		{"strip_outer_tildes", "~Global(\"X\",\"GLOBAL\",1)~", `Global("X","GLOBAL",1)`},
		{"strip_outer_tildes_keep_inner", "~~Global(\"X\",\"GLOBAL\",1)~~", `~Global("X","GLOBAL",1)~`},
		{"strip_outer_tildes_and_trim_inside", "~  a == b   ~", "a == b"},
		{"no_outer_tildes_passthrough", `Global("X","GLOBAL",1)`, `Global("X","GLOBAL",1)`},
		{"single_leading_tilde_passthrough", "~abc", "~abc"},
		{"single_trailing_tilde_passthrough", "abc~", "abc~"},
		{"just_one_tilde_passthrough", "~", "~"},
		{"tildes_with_only_spaces_inside", "~   ~", ""}, // trims to ""
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCondition(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeCondition(%q)=%q want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLooksLikeWeiduCode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"spaces_only", "   \n\t ", false},

		{"if_upper", "IF ~~ THEN REPLY @1 EXIT", true},
		{"if_lower", "if ~~ then reply @1 exit", true},
		{"if_with_leading_spaces", "   IF ~~ THEN", true},

		{"chain", "CHAIN IF ~~ THEN X", true},
		{"begin", "BEGIN AC#TEST", true},
		{"say", "SAY @100", true},
		{"do", "DO ~SetGlobal(\"X\",\"GLOBAL\",1)~", true},

		{"double_equals", "== SomeOtherDlg", true},
		{"at_strref", "@123", true},
		{"extern", "EXTERN AC#TEST NEXT", true},
		{"exit", "EXIT", true},

		// should be false for normal text / non-matching prefixes
		{"plain_text", "Hello there.", false},
		{"tilde_condition", "~Global(\"X\",\"GLOBAL\",1)~", false},
		{"starts_with_iff_not_if", "iff something", false},
		{"starts_with_beginning_not_begin", "beginning of something", false}, // because prefix is "BEGIN", not "BEGINNING"
		{"starts_with_doctor_not_do", "doctor who", false},                   // prefix is "DO", not "DOCTOR" but NOTE: strings.HasPrefix("DOCTOR","DO") == true -> this WOULD be true
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikeWeiduCode(tc.in)
			if got != tc.want {
				t.Fatalf("looksLikeWeiduCode(%q)=%v want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIntPtrAndStrPtr(t *testing.T) {
	t.Run("intPtr", func(t *testing.T) {
		p := intPtr(42)
		if p == nil || *p != 42 {
			t.Fatalf("intPtr(42)=%v, want ptr to 42", p)
		}
	})

	t.Run("strPtr", func(t *testing.T) {
		p := strPtr("x")
		if p == nil || *p != "x" {
			t.Fatalf("strPtr(%q)=%v, want ptr to %q", "x", p, "x")
		}
	})
}
