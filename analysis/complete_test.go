package analysis

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// probed returns the snapshot of text with the probe at |.
func probed(path, text string) *psi.Snapshot {
	return psi.Parse(path, strings.Replace(text, "|", ProbeName, 1))
}

func itemNames(c *Completion) []string {
	var out []string
	for _, it := range c.Items {
		out = append(out, it.Name)
	}
	return out
}

func findItem(c *Completion, name string) *CompletionItem {
	for i := range c.Items {
		if c.Items[i].Name == name {
			return &c.Items[i]
		}
	}
	return nil
}

const completeLib = `package p

/** A box. */
class Box<T>(val item: T) {
    fun take(): T = item
    fun put(x: T, times: Int = 1) {}
}

fun topLevel(n: Int): String = n.toString()
`

func TestComplete(t *testing.T) {
	cfg := withJDK(t)
	s := Open(cfg)
	defer s.Close()
	ctx := context.Background()
	lib := psi.Parse("lib.kt", completeLib)
	use := "package p\n\nfun main() {\n    val count = 1\n    val box = Box(\"s\")\n    %s\n}\n"
	first := []*psi.Snapshot{lib, psi.Parse("use.kt", strings.Replace(use, "%s", "println(count)", 1))}
	if _, err := s.Update(ctx, first); err != nil {
		t.Fatal(err)
	}

	complete := func(stmt, prefix string) *Completion {
		t.Helper()
		snaps := []*psi.Snapshot{lib, probed("use.kt", strings.Replace(use, "%s", stmt, 1))}
		c, err := s.Complete(ctx, snaps, "use.kt", prefix)
		if err != nil {
			t.Fatal(err)
		}
		if !c.Reached {
			t.Fatalf("%s: probe not reached", stmt)
		}
		return c
	}

	t.Run("scope", func(t *testing.T) {
		c := complete("|", "co")
		names := itemNames(c)
		for _, want := range []string{"count", "box", "main", "topLevel", "Box", "println", "listOf", "String"} {
			if !slices.Contains(names, want) {
				t.Errorf("no %s in %v", want, names)
			}
		}
		if i, j := slices.Index(names, "count"), slices.Index(names, "topLevel"); i > j {
			t.Errorf("count after topLevel: %v", names)
		}
		it := findItem(c, "count")
		if it.Kind != "local" || it.Type == nil || it.Type.Name != "kotlin.Int" || it.Declaration == nil {
			t.Errorf("count = %+v", it)
		} else if n := c.Node(*it.Declaration); !strings.HasPrefix(n.Text(), "val count") {
			t.Errorf("count declared at %q", n.Text())
		}
		if box := findItem(c, "Box"); box.Kind != "class" || box.QualifiedName != "p.Box" || box.Declaration == nil || box.Call == nil {
			t.Errorf("Box = %+v", box)
		}
		if !slices.ContainsFunc(c.Importables, func(im Importable) bool {
			return im.Name == "ConcurrentHashMap" && im.Package == "java.util.concurrent" && im.Class
		}) {
			t.Errorf("no ConcurrentHashMap among %d importables", len(c.Importables))
		}
		for _, im := range c.Importables {
			if r := im.Name[0] | 0x20; r != 'c' {
				t.Fatalf("importable %+v for prefix co", im)
			}
		}
	})

	t.Run("members", func(t *testing.T) {
		c := complete("box.|", "")
		if c.Receiver == nil || c.Receiver.Name != "p.Box" {
			t.Fatalf("receiver %+v", c.Receiver)
		}
		take := findItem(c, "take")
		if take == nil || take.Origin != "member" || take.Kind != "function" || take.Type == nil || take.Type.Name != "kotlin.String" {
			t.Fatalf("take = %+v", take)
		}
		put := findItem(c, "put")
		if put == nil || put.Call == nil || len(put.Call.Parameters) != 2 || put.Call.Parameters[0].Name != "kotlin.String" || put.Call.ParameterNames[1] != "times" {
			t.Errorf("put = %+v", put)
		}
		if item := findItem(c, "item"); item == nil || item.Kind != "property" {
			t.Errorf("item = %+v", item)
		}
		if let := findItem(c, "let"); let == nil || let.Origin != "extension" {
			t.Errorf("let = %+v", let)
		}
		if len(c.Importables) != 0 {
			t.Errorf("importables after a receiver")
		}
	})

	t.Run("call", func(t *testing.T) {
		c := complete("box.put(\"a\", |)", "")
		if len(c.Calls) == 0 {
			t.Fatal("no calls")
		}
		cc := c.Calls[0]
		if cc.Argument != 1 || len(cc.Candidates) != 1 || cc.Candidates[0].Name != "p.Box.put" {
			t.Errorf("call %+v", cc)
		}
		if c.Expected == nil || c.Expected.Name != "kotlin.Int" {
			t.Errorf("expected %+v", c.Expected)
		}
	})

	t.Run("type", func(t *testing.T) {
		c := complete("val x: | = 1", "Bo")
		if !c.TypePosition || findItem(c, "Box") == nil || findItem(c, "count") != nil {
			t.Errorf("type position %v, items %v", c.TypePosition, itemNames(c))
		}
		for _, im := range c.Importables {
			if !im.Class {
				t.Fatalf("importable %+v in a type position", im)
			}
		}
	})

	t.Run("not committed", func(t *testing.T) {
		// The session's next update is still incremental from the first.
		snaps := []*psi.Snapshot{lib, psi.Parse("use.kt", strings.Replace(use, "%s", "println(count + 1)", 1))}
		s.mu.Lock()
		last := s.last
		s.mu.Unlock()
		if last == nil || last.snaps[1] != first[1] {
			t.Fatal("Complete changed the session's state")
		}
		got, err := s.Update(ctx, snaps)
		if err != nil {
			t.Fatal(err)
		}
		if d := sameResult(t, got, wholeUpdate(t, cfg, snaps)); d != "" {
			t.Error(d)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		snaps := []*psi.Snapshot{lib, probed("use.kt", strings.Replace(use, "%s", "|", 1))}
		if _, err := s.Complete(ctx, snaps, "use.kt", ""); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v", err)
		}
	})
}
