package d

import (
	"strings"
	"testing"
)

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

func TestParseReader_BeginWithTildes(t *testing.T) {
	input := `
BEGIN ~AC#TEST~

IF ~~ THEN BEGIN START
  SAY @1
  IF ~~ THEN REPLY @2 EXIT
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	if len(occ) != 2 {
		t.Fatalf("expected 2 occurrences, got %d: %+v", len(occ), occ)
	}
	if occ[0].Dialog != "AC#TEST" || occ[0].State != "START" {
		t.Fatalf("unexpected dialog/state: %+v", occ[0])
	}
}

func TestParseReader_StateEntryCondition_PropagatesToSayAndContinuation(t *testing.T) {
	input := `
BEGIN AC#TEST

IF ~Global("X","GLOBAL",1)~ THEN BEGIN A
  SAY @1
  = @2
  @3
  IF ~~ THEN REPLY @10 EXIT
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	// Expect: SAY@1, @2, @3, REPLY@10 => 4
	if len(occ) != 4 {
		t.Fatalf("expected 4 occurrences, got %d: %+v", len(occ), occ)
	}

	for i := 0; i < 3; i++ {
		if occ[i].Kind != KindNPC {
			t.Fatalf("occ[%d] expected NPC, got: %+v", i, occ[i])
		}
		if occ[i].Condition != `Global("X","GLOBAL",1)` {
			t.Fatalf("occ[%d] expected stateEntryCond to propagate, got: %q", i, occ[i].Condition)
		}
	}
}

func TestParseReader_NotesAccumulateAcrossCommentOnlyLinesAndInline(t *testing.T) {
	input := `
BEGIN AC#TEST

// a
// b
IF ~~ THEN BEGIN A
  // c
  SAY @1 // d
  IF ~~ THEN REPLY @2 EXIT // e
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}

	if len(occ) != 2 {
		t.Fatalf("expected 2 occurrences, got %d: %+v", len(occ), occ)
	}

	// Notes attach to the next emitted occurrence and then reset.
	// For SAY: should include a,b,c,d
	want0 := []string{"a", "b", "c", "d"}
	if !equalStringSlices(occ[0].Notes, want0) {
		t.Fatalf("occ[0].Notes mismatch:\n got: %#v\nwant: %#v", occ[0].Notes, want0)
	}

	// For REPLY: should include e only
	want1 := []string{"e"}
	if !equalStringSlices(occ[1].Notes, want1) {
		t.Fatalf("occ[1].Notes mismatch:\n got: %#v\nwant: %#v", occ[1].Notes, want1)
	}
}

func TestParseReader_SplitLineComment_IgnoresDoubleSlashInsideTilde(t *testing.T) {
	input := `
BEGIN AC#TEST

IF ~~ THEN BEGIN A
  // note before say
  SAY @1 // outside comment ok
  IF ~Global("X","GLOBAL",1)//not_comment~ THEN REPLY @2 EXIT // real comment
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	if len(occ) != 2 {
		t.Fatalf("expected 2 occurrences, got %d: %+v", len(occ), occ)
	}

	if occ[1].Condition != `Global("X","GLOBAL",1)//not_comment` {
		t.Fatalf("expected condition to preserve // inside ~ ~, got: %q", occ[1].Condition)
	}
	// Also ensure inline comment captured
	want := []string{"real comment"}
	if !equalStringSlices(occ[1].Notes, want) {
		t.Fatalf("occ[1].Notes mismatch:\n got: %#v\nwant: %#v", occ[1].Notes, want)
	}
}

func TestParseReader_CommentedOutWeiduCode_IsIgnoredAsNote(t *testing.T) {
	input := `
BEGIN AC#TEST

IF ~~ THEN BEGIN A
  //IF ~~ THEN REPLY @999 EXIT
  //==FOO @123
  //@777
  SAY @1
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	if len(occ) != 1 {
		t.Fatalf("expected 1 occurrence, got %d: %+v", len(occ), occ)
	}
	if len(occ[0].Notes) != 0 {
		t.Fatalf("expected no notes (commented-out code ignored), got: %#v", occ[0].Notes)
	}
}

func TestParseReader_ReplyTargets_GOTO_Works(t *testing.T) {
	input := `
BEGIN AC#TEST

IF ~~ THEN BEGIN A
  SAY @1
  IF ~~ THEN REPLY @10 GOTO B
END
IF ~~ THEN BEGIN B
  SAY @2
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	if len(occ) != 3 {
		t.Fatalf("expected 3 occurrences, got %d: %+v", len(occ), occ)
	}

	reply := occ[1]
	if reply.Kind != KindPC || reply.ToType != "GOTO" {
		t.Fatalf("expected GOTO reply, got: %+v", reply)
	}
	if reply.ToDlg == nil || *reply.ToDlg != "AC#TEST" || reply.ToState == nil || *reply.ToState != "B" {
		t.Fatalf("GOTO target mismatch: %+v", reply)
	}
}

func TestParseReader_ReplyTargets_ExternParsing_CaseAndSpacing(t *testing.T) {
	input := `
BEGIN AC#TEST
IF ~~ THEN BEGIN A
  SAY @1
  IF ~~ THEN REPLY @10    extern   AC#TEST   B
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	if len(occ) != 2 {
		t.Fatalf("expected 2 occurrences, got %d: %+v", len(occ), occ)
	}
	r := occ[1]
	if r.ToType != "EXTERN" || r.ToDlg == nil || *r.ToDlg != "AC#TEST" || r.ToState == nil || *r.ToState != "B" {
		t.Fatalf("extern target mismatch: %+v", r)
	}
}

func TestParseReader_ChainBody_CanEndWithoutEND_OnExtern(t *testing.T) {
	input := `
BEGIN AC#TEST

CHAIN AC#TEST A
@1
EXTERN AC#TEST B

CHAIN AC#TEST B
@2
END
IF ~~ THEN REPLY @10 EXIT
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}

	// Expect: @1 (with auto EXTERN), @2, reply@10 => 3
	if len(occ) != 3 {
		t.Fatalf("expected 3 occurrences, got %d: %+v", len(occ), occ)
	}
	if occ[0].ToType != "EXTERN" || occ[0].ToDlg == nil || *occ[0].ToDlg != "AC#TEST" || occ[0].ToState == nil || *occ[0].ToState != "B" {
		t.Fatalf("expected auto EXTERN on first chain text, got: %+v", occ[0])
	}
	// Ensure we got to parse next CHAIN + reply afterwards.
	if occ[2].Kind != KindPC || occ[2].ToType != "EXIT" {
		t.Fatalf("expected final reply EXIT, got: %+v", occ[2])
	}
}

func TestParseReader_ChainBody_CanEndWithoutEND_OnExit(t *testing.T) {
	input := `
BEGIN AC#TEST

CHAIN AC#TEST A
@1
EXIT

IF ~~ THEN BEGIN X
  SAY @2
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	// Expect @1 (auto EXIT), then SAY @2 => 2
	if len(occ) != 2 {
		t.Fatalf("expected 2 occurrences, got %d: %+v", len(occ), occ)
	}
	if occ[0].ToType != "EXIT" {
		t.Fatalf("expected auto EXIT, got: %+v", occ[0])
	}
}

func TestParseReader_ChainHeader_BufferingMultilineIFThen(t *testing.T) {
	input := `
BEGIN AC#TEST

CHAIN IF ~Global("X","GLOBAL",1)
  && Global("Y","GLOBAL",2)~ THEN AC#TEST A
@1
END
IF ~~ THEN REPLY @10 EXIT
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	// Expect @1 + reply => 2
	if len(occ) != 2 {
		t.Fatalf("expected 2 occurrences, got %d: %+v", len(occ), occ)
	}
	if occ[0].Kind != KindNPC || occ[0].State != "A" || occ[0].Dialog != "AC#TEST" {
		t.Fatalf("unexpected first occurrence: %+v", occ[0])
	}
}

func TestParseReader_ExtendBottom_TreatedAsInStateAndClosesOnEND(t *testing.T) {
	input := `
EXTEND_BOTTOM ~PGOND~ 0
  SAY @1
  IF ~~ THEN REPLY @10 GOTO X
END

BEGIN AC#TEST
IF ~~ THEN BEGIN A
  SAY @2
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}

	// EXTEND: SAY @1, REPLY @10; then BEGIN: SAY @2 => 3
	if len(occ) != 3 {
		t.Fatalf("expected 3 occurrences, got %d: %+v", len(occ), occ)
	}

	if occ[0].Dialog != "PGOND" || occ[0].State != "0" {
		t.Fatalf("extend dialog/state mismatch: %+v", occ[0])
	}
	if occ[1].Dialog != "PGOND" || occ[1].State != "0" || occ[1].ToType != "GOTO" {
		t.Fatalf("extend reply mismatch: %+v", occ[1])
	}

	// Ensure END closed extend and didn't leak into following BEGIN
	if occ[2].Dialog != "AC#TEST" || occ[2].State != "A" {
		t.Fatalf("after extend, expected AC#TEST/A, got: %+v", occ[2])
	}
}

func TestParseReader_ExtendTop_ParsesSameAsBottom(t *testing.T) {
	input := `
EXTEND_TOP ~ABC~ some_state
  SAY @1
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	if len(occ) != 1 {
		t.Fatalf("expected 1 occurrence, got %d: %+v", len(occ), occ)
	}
	if occ[0].Dialog != "ABC" || occ[0].State != "some_state" {
		t.Fatalf("extend top dialog/state mismatch: %+v", occ[0])
	}
}

func TestParseReader_ModeNormal_EndDoesNotCloseRegularState(t *testing.T) {
	// In modeNormal, "END" only closes EXTEND_* (per code). Regular states close only in modeState.
	input := `
BEGIN AC#TEST

IF ~~ THEN BEGIN A
  SAY @1
END
IF ~~ THEN REPLY @10 EXIT
`
	_, err := ParseReader(strings.NewReader(input), "x.d")
	if err == nil {
		t.Fatalf("expected error: REPLY outside state because END in modeNormal doesn't close state blocks")
	}
	// error string should contain "REPLY outside state"
	if !strings.Contains(err.Error(), "REPLY outside state") {
		t.Fatalf("expected REPLY outside state error, got: %v", err)
	}
}

func TestParseReader_Err_StateBeforeBegin(t *testing.T) {
	input := `
IF ~~ THEN BEGIN A
  SAY @1
END
`
	_, err := ParseReader(strings.NewReader(input), "x.d")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "state defined before BEGIN") {
		t.Fatalf("expected state defined before BEGIN, got: %v", err)
	}
}

func TestParseReader_Err_SayOutsideState(t *testing.T) {
	input := `
BEGIN AC#TEST
SAY @1
`
	_, err := ParseReader(strings.NewReader(input), "x.d")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "SAY outside state") {
		t.Fatalf("expected SAY outside state, got: %v", err)
	}
}

func TestParseReader_Err_ExternInChainBodyWithoutText(t *testing.T) {
	input := `
BEGIN AC#TEST
CHAIN AC#TEST A
EXTERN AC#TEST B
`
	_, err := ParseReader(strings.NewReader(input), "x.d")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "EXTERN in CHAIN body without preceding text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseReader_Err_ExitInChainBodyWithoutText(t *testing.T) {
	input := `
BEGIN AC#TEST
CHAIN AC#TEST A
EXIT
`
	_, err := ParseReader(strings.NewReader(input), "x.d")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "EXIT in CHAIN body without preceding text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseReader_StateMode_IgnoresDoExitLine(t *testing.T) {
	input := `
BEGIN AC#TEST
IF ~~ THEN BEGIN A
  SAY @1
  IF ~~ THEN DO ~SetGlobal("X","GLOBAL",1)~ EXIT
  IF ~~ THEN REPLY @10 EXIT
END
`
	occ, err := ParseReader(strings.NewReader(input), "x.d")
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	// DO...EXIT should not create occurrence; we still get SAY + REPLY => 2
	if len(occ) != 2 {
		t.Fatalf("expected 2 occurrences, got %d: %+v", len(occ), occ)
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
