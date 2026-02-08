package d

import (
	"strings"
	"testing"
)

func iptr(v int) *int       { return &v }
func sptr(v string) *string { return &v }

func TestParseReader_SimpleDialog(t *testing.T) {
	input := `
BEGIN AC#TEST

// state gating note
IF ~Global("AC#X","GLOBAL",1)~ THEN BEGIN START
  SAY @100 // npc greet
  IF ~~ THEN REPLY @110 EXTERN AC#TEST NEXT // go next
  IF ~~ THEN REPLY @120 EXIT
END

IF ~~ THEN BEGIN NEXT
  SAY @102
  DO ~SetGlobal("AC#X","GLOBAL",2)~ EXIT
END
`

	occ, err := ParseReader(strings.NewReader(input), "AC#TEST.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}

	// we want 4 records:
	// - SAY @100
	// - REPLY @110 -> EXTERN AC#TEST NEXT
	// - REPLY @120 -> EXIT
	// - SAY @102
	if len(occ) != 4 {
		t.Fatalf("expected 4 occurrences, got %d: %+v", len(occ), occ)
	}

	// 0) SAY @100 (+ notes: comment line above + inline comment)
	if occ[0].Kind != KindNPC || occ[0].TraID == nil || *occ[0].TraID != 100 {
		t.Fatalf("occ[0] expected NPC @100, got: %+v", occ[0])
	}
	if occ[0].Dialog != "AC#TEST" || occ[0].State != "START" || occ[0].SpeakerDlg != "AC#TEST" {
		t.Fatalf("occ[0] dialog/state/speaker mismatch: %+v", occ[0])
	}
	// notes: "// state gating note" and "// npc greet"
	wantNotes0 := []string{"state gating note", "npc greet"}
	if !equalStringSlices(occ[0].Notes, wantNotes0) {
		t.Fatalf("occ[0].Notes mismatch:\n got: %#v\nwant: %#v", occ[0].Notes, wantNotes0)
	}

	// 1) REPLY @110 EXTERN AC#TEST NEXT (replyIndex 0)
	if occ[1].Kind != KindPC || occ[1].TraID == nil || *occ[1].TraID != 110 {
		t.Fatalf("occ[1] expected PC @110, got: %+v", occ[1])
	}
	if occ[1].ReplyIndex == nil || *occ[1].ReplyIndex != 0 {
		t.Fatalf("occ[1] expected ReplyIndex=0, got: %+v", occ[1])
	}
	if occ[1].ToType != "EXTERN" || occ[1].ToDlg == nil || *occ[1].ToDlg != "AC#TEST" || occ[1].ToState == nil || *occ[1].ToState != "NEXT" {
		t.Fatalf("occ[1] extern target mismatch: %+v", occ[1])
	}
	// inline comment in same line
	wantNotes1 := []string{"go next"}
	if !equalStringSlices(occ[1].Notes, wantNotes1) {
		t.Fatalf("occ[1].Notes mismatch:\n got: %#v\nwant: %#v", occ[1].Notes, wantNotes1)
	}

	// 2) REPLY @120 EXIT (replyIndex 1)
	if occ[2].Kind != KindPC || occ[2].TraID == nil || *occ[2].TraID != 120 {
		t.Fatalf("occ[2] expected PC @120, got: %+v", occ[2])
	}
	if occ[2].ReplyIndex == nil || *occ[2].ReplyIndex != 1 {
		t.Fatalf("occ[2] expected ReplyIndex=1, got: %+v", occ[2])
	}
	if occ[2].ToType != "EXIT" {
		t.Fatalf("occ[2] expected EXIT, got: %+v", occ[2])
	}

	// 3) SAY @102 in NEXT
	if occ[3].Kind != KindNPC || occ[3].TraID == nil || *occ[3].TraID != 102 {
		t.Fatalf("occ[3] expected NPC @102, got: %+v", occ[3])
	}
	if occ[3].State != "NEXT" {
		t.Fatalf("occ[3] expected State NEXT, got: %+v", occ[3])
	}
}

func TestParseReader_ReplyIndexResetsPerState(t *testing.T) {
	input := `
BEGIN AC#TEST

IF ~~ THEN BEGIN A
  SAY @1
  IF ~~ THEN REPLY @10 EXIT
  IF ~~ THEN REPLY @11 EXIT
END

IF ~~ THEN BEGIN B
  SAY @2
  IF ~~ THEN REPLY @20 EXIT
END
`
	occ, err := ParseReader(strings.NewReader(input), "AC#TEST.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}

	var gotA, gotB []int
	for _, o := range occ {
		if o.Kind != KindPC || o.ReplyIndex == nil {
			continue
		}
		if o.State == "A" {
			gotA = append(gotA, *o.ReplyIndex)
		}
		if o.State == "B" {
			gotB = append(gotB, *o.ReplyIndex)
		}
	}

	if !equalIntSlices(gotA, []int{0, 1}) {
		t.Fatalf("replyIndex in state A mismatch: got=%v want=%v", gotA, []int{0, 1})
	}
	if !equalIntSlices(gotB, []int{0}) {
		t.Fatalf("replyIndex in state B mismatch: got=%v want=%v", gotB, []int{0})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
