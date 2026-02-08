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

func TestParseReader_ChainDialog(t *testing.T) {
	input := `
// 02_Dialog.d - test cases for dlg2csv (CHAIN + interjections + replies + DO + multiple BEGIN)

BEGIN AC#WOMAN

CHAIN AC#WOMAN HELLO
@200
==JAHEIJ IF ~InParty("JAHEIRA")~ THEN @201
==AC#WOMAN @204
END
IF~~THEN REPLY @210 EXTERN AC#WOMAN CONT
//IF~~THEN REPLY @999 EXTERN AC#WOMAN NOPE
IF ~~ THEN REPLY @220 EXTERN AC#WOMAN BYE
IF~~THEN REPLY @222 DO ~SetGlobal("AC#X","GLOBAL",1)~ EXTERN AC#WOMAN CONT

CHAIN AC#WOMAN CONT
@202
EXTERN AC#WOMAN BYE

CHAIN AC#WOMAN BYE
@203
EXIT


// Additional BEGIN in the same .d file
BEGIN AC#OTHER

CHAIN IF ~True()~ THEN AC#OTHER START
@400
END
IF~~THEN REPLY @410 EXIT
`

	occ, err := ParseReader(strings.NewReader(input), "02_Dialog.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}

	// Expected occurrences (text lines + replies). We attach EXTERN/EXIT inside CHAIN bodies
	// to the last emitted NPC text in that CHAIN, so they don't create extra occurrences.
	//
	// AC#WOMAN HELLO:
	//   NPC @200 (AC#WOMAN)
	//   NPC @201 (JAHEIJ, cond)
	//   NPC @204 (AC#WOMAN)
	//   PC  @210 -> EXTERN AC#WOMAN CONT  (replyIndex 0)
	//   PC  @220 -> EXTERN AC#WOMAN BYE   (replyIndex 1)
	//   PC  @222 -> EXTERN AC#WOMAN CONT  (replyIndex 2)
	//
	// AC#WOMAN CONT:
	//   NPC @202 (auto EXTERN -> AC#WOMAN BYE)
	//
	// AC#WOMAN BYE:
	//   NPC @203 (auto EXIT)
	//
	// AC#OTHER START:
	//   NPC @400
	//   PC  @410 -> EXIT (replyIndex 0)
	//
	// Total: 10 occurrences
	if len(occ) != 10 {
		t.Fatalf("expected 10 occurrences, got %d: %+v", len(occ), occ)
	}

	// 0) @200 NPC AC#WOMAN HELLO
	if occ[0].Kind != KindNPC || occ[0].TraID == nil || *occ[0].TraID != 200 {
		t.Fatalf("occ[0] expected NPC @200, got: %+v", occ[0])
	}
	if occ[0].Dialog != "AC#WOMAN" || occ[0].State != "HELLO" || occ[0].SpeakerDlg != "AC#WOMAN" {
		t.Fatalf("occ[0] dialog/state/speaker mismatch: %+v", occ[0])
	}

	// 1) @201 NPC interjection by JAHEIJ with condition
	if occ[1].Kind != KindNPC || occ[1].TraID == nil || *occ[1].TraID != 201 {
		t.Fatalf("occ[1] expected NPC @201, got: %+v", occ[1])
	}
	if occ[1].Dialog != "AC#WOMAN" || occ[1].State != "HELLO" || occ[1].SpeakerDlg != "JAHEIJ" {
		t.Fatalf("occ[1] dialog/state/speaker mismatch: %+v", occ[1])
	}
	if occ[1].Condition != `InParty("JAHEIRA")` {
		t.Fatalf("occ[1] expected condition InParty(\"JAHEIRA\"), got: %q", occ[1].Condition)
	}

	// 2) @204 NPC interjection by AC#WOMAN
	if occ[2].Kind != KindNPC || occ[2].TraID == nil || *occ[2].TraID != 204 {
		t.Fatalf("occ[2] expected NPC @204, got: %+v", occ[2])
	}
	if occ[2].Dialog != "AC#WOMAN" || occ[2].State != "HELLO" || occ[2].SpeakerDlg != "AC#WOMAN" {
		t.Fatalf("occ[2] dialog/state/speaker mismatch: %+v", occ[2])
	}

	// 3) @210 PC replyIndex 0 -> EXTERN AC#WOMAN CONT
	if occ[3].Kind != KindPC || occ[3].TraID == nil || *occ[3].TraID != 210 {
		t.Fatalf("occ[3] expected PC @210, got: %+v", occ[3])
	}
	if occ[3].ReplyIndex == nil || *occ[3].ReplyIndex != 0 {
		t.Fatalf("occ[3] expected ReplyIndex=0, got: %+v", occ[3])
	}
	if occ[3].ToType != "EXTERN" || occ[3].ToDlg == nil || *occ[3].ToDlg != "AC#WOMAN" || occ[3].ToState == nil || *occ[3].ToState != "CONT" {
		t.Fatalf("occ[3] extern target mismatch: %+v", occ[3])
	}

	// 4) @220 PC replyIndex 1 -> EXTERN AC#WOMAN BYE
	if occ[4].Kind != KindPC || occ[4].TraID == nil || *occ[4].TraID != 220 {
		t.Fatalf("occ[4] expected PC @220, got: %+v", occ[4])
	}
	if occ[4].ReplyIndex == nil || *occ[4].ReplyIndex != 1 {
		t.Fatalf("occ[4] expected ReplyIndex=1, got: %+v", occ[4])
	}
	if occ[4].ToType != "EXTERN" || occ[4].ToDlg == nil || *occ[4].ToDlg != "AC#WOMAN" || occ[4].ToState == nil || *occ[4].ToState != "BYE" {
		t.Fatalf("occ[4] extern target mismatch: %+v", occ[4])
	}

	// 5) @222 PC replyIndex 2 -> EXTERN AC#WOMAN CONT (DO ... should not break target parsing)
	if occ[5].Kind != KindPC || occ[5].TraID == nil || *occ[5].TraID != 222 {
		t.Fatalf("occ[5] expected PC @222, got: %+v", occ[5])
	}
	if occ[5].ReplyIndex == nil || *occ[5].ReplyIndex != 2 {
		t.Fatalf("occ[5] expected ReplyIndex=2, got: %+v", occ[5])
	}
	if occ[5].ToType != "EXTERN" || occ[5].ToDlg == nil || *occ[5].ToDlg != "AC#WOMAN" || occ[5].ToState == nil || *occ[5].ToState != "CONT" {
		t.Fatalf("occ[5] extern target mismatch: %+v", occ[5])
	}

	// 6) @202 NPC in CONT with auto EXTERN -> AC#WOMAN BYE
	if occ[6].Kind != KindNPC || occ[6].TraID == nil || *occ[6].TraID != 202 {
		t.Fatalf("occ[6] expected NPC @202, got: %+v", occ[6])
	}
	if occ[6].Dialog != "AC#WOMAN" || occ[6].State != "CONT" || occ[6].SpeakerDlg != "AC#WOMAN" {
		t.Fatalf("occ[6] dialog/state/speaker mismatch: %+v", occ[6])
	}
	if occ[6].ToType != "EXTERN" || occ[6].ToDlg == nil || *occ[6].ToDlg != "AC#WOMAN" || occ[6].ToState == nil || *occ[6].ToState != "BYE" {
		t.Fatalf("occ[6] expected auto EXTERN to AC#WOMAN BYE, got: %+v", occ[6])
	}

	// 7) @203 NPC in BYE with auto EXIT
	if occ[7].Kind != KindNPC || occ[7].TraID == nil || *occ[7].TraID != 203 {
		t.Fatalf("occ[7] expected NPC @203, got: %+v", occ[7])
	}
	if occ[7].Dialog != "AC#WOMAN" || occ[7].State != "BYE" || occ[7].SpeakerDlg != "AC#WOMAN" {
		t.Fatalf("occ[7] dialog/state/speaker mismatch: %+v", occ[7])
	}
	if occ[7].ToType != "EXIT" {
		t.Fatalf("occ[7] expected auto EXIT, got: %+v", occ[7])
	}

	// 8) @400 NPC in AC#OTHER START (CHAIN IF ... THEN ...)
	if occ[8].Kind != KindNPC || occ[8].TraID == nil || *occ[8].TraID != 400 {
		t.Fatalf("occ[8] expected NPC @400, got: %+v", occ[8])
	}
	if occ[8].Dialog != "AC#OTHER" || occ[8].State != "START" || occ[8].SpeakerDlg != "AC#OTHER" {
		t.Fatalf("occ[8] dialog/state/speaker mismatch: %+v", occ[8])
	}

	// 9) @410 PC in AC#OTHER START replyIndex 0 -> EXIT
	if occ[9].Kind != KindPC || occ[9].TraID == nil || *occ[9].TraID != 410 {
		t.Fatalf("occ[9] expected PC @410, got: %+v", occ[9])
	}
	if occ[9].ReplyIndex == nil || *occ[9].ReplyIndex != 0 {
		t.Fatalf("occ[9] expected ReplyIndex=0, got: %+v", occ[9])
	}
	if occ[9].ToType != "EXIT" {
		t.Fatalf("occ[9] expected EXIT, got: %+v", occ[9])
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
