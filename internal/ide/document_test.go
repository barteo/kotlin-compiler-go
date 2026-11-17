package ide

import "testing"

func TestPositions(t *testing.T) {
	// "é" is 2 UTF-8 bytes and 1 UTF-16 unit; "😀" is 4 bytes and 2 units.
	d := NewDocument("A.kt", 1, "val é = 1\r\nval 😀x = 2\n")
	for _, c := range []struct {
		off  int
		u16  Pos
		utf8 Pos
	}{
		{0, Pos{0, 0}, Pos{0, 0}},
		{6, Pos{0, 5}, Pos{0, 6}},  // after "é"
		{12, Pos{1, 0}, Pos{1, 0}}, // after "\r\n"
		{20, Pos{1, 6}, Pos{1, 8}}, // after "😀"
		{26, Pos{2, 0}, Pos{2, 0}}, // end of text
	} {
		for _, e := range []struct {
			enc Encoding
			p   Pos
		}{{UTF16, c.u16}, {UTF8, c.utf8}} {
			if got := d.Pos(c.off, e.enc); got != e.p {
				t.Errorf("Pos(%d, %d) = %v, want %v", c.off, e.enc, got, e.p)
			}
			if got, err := d.Offset(e.p, e.enc); err != nil || got != c.off {
				t.Errorf("Offset(%v, %d) = %d, %v, want %d", e.p, e.enc, got, err, c.off)
			}
		}
	}
	// Inside a rune: its start. Past the line's end: the line's end, before "\r\n".
	if got := d.Pos(17, UTF16); got != (Pos{1, 4}) {
		t.Errorf("Pos inside rune = %v", got)
	}
	if got, _ := d.Offset(Pos{1, 5}, UTF16); got != 16 {
		t.Errorf("Offset inside surrogate pair = %d, want 16", got)
	}
	if got, _ := d.Offset(Pos{0, 99}, UTF16); got != 10 {
		t.Errorf("Offset past line end = %d, want 10", got)
	}
	if _, err := d.Offset(Pos{5, 0}, UTF16); err == nil {
		t.Error("Offset past the document: no error")
	}
}

func TestApply(t *testing.T) {
	d := NewDocument("A.kt", 1, "fun f() = 1\n")
	d2, err := d.Apply(2, []Edit{
		{From: Pos{0, 4}, To: Pos{0, 5}, Text: "😀"},
		{From: Pos{0, 6}, To: Pos{0, 6}, Text: "g"}, // after the surrogate pair
		{From: Pos{1, 0}, To: Pos{1, 0}, Text: "val x = 2\n"},
	}, UTF16)
	if err != nil {
		t.Fatal(err)
	}
	if want := "fun 😀g() = 1\nval x = 2\n"; d2.Text() != want || d2.Version != 2 {
		t.Errorf("got %q v%d, want %q v2", d2.Text(), d2.Version, want)
	}
	if d.Text() != "fun f() = 1\n" {
		t.Error("Apply changed the old document")
	}
	if _, err := d.Apply(3, []Edit{{From: Pos{0, 5}, To: Pos{0, 4}}}, UTF16); err == nil {
		t.Error("reversed range: no error")
	}
	d3, _ := d2.Apply(3, []Edit{{Whole: true, Text: "class C"}}, UTF8)
	if d3.Text() != "class C" || len(d3.Snap.Diagnostics()) != 0 {
		t.Errorf("whole replace: %q", d3.Text())
	}
}
