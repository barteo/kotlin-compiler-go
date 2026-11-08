package lower

import (
	"fmt"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestRecover checks that Recover keeps the declarations around a broken
// statement or expression, and the statements before it, where File
// drops the declaration.
func TestRecover(t *testing.T) {
	tests := []struct {
		name, code string
		// want is f's body as summarize gives it, or "dropped".
		want string
	}{
		// The next line is foo's argument, and the condition.
		{"argument", "foo(1, ", "var x; error"},
		{"condition", "if (", "var x; error"},
		{"initializer", "val y = )", "var x; var y=error; error; call"},
		{"member", "x.;", "var x; error; call"},
		{"stray", ")", "var x; error; call"},
		{"two", "x = = 1", "var x; error; error; call"},
		{"nested", "if (x > 0) { foo(1, }", "var x; if; call"},
		{"lambda", "run { foo(1, }", "var x; call; call"},
		{"string", `val s = "abc`, "var x; var s=error; call"},
		// psi swallows f's closing brace, and b with it.
		{"unclosed lambda", "run {", "dropped"},
		{"unclosed string", `if (x > 0) { val s = "abc }`, "dropped"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "fun a() = 1\n\nfun f() {\n    val x = 1\n    " + tt.code + "\n    println(x)\n}\n\nfun b() = 2\n"
			s := psi.Parse("a.kt", src)
			if _, _, errs := File(s); len(errs) == 0 {
				t.Fatal("File: no error")
			}
			f, _, errs := Recover(s)
			if len(errs) == 0 {
				t.Error("Recover: no error")
			}
			got := map[string]string{}
			for _, d := range f.Decls {
				if fd, ok := d.(*syntax.FunDecl); ok {
					got[fd.Name] = summarize(fd)
				}
			}
			if got["a"] == "" || got["b"] == "" && tt.want != "dropped" {
				t.Errorf("declarations %v: a or b dropped", got)
			}
			if body, ok := got["f"]; !ok && tt.want != "dropped" || ok && body != tt.want {
				t.Errorf("f: %q, want %q (errors %v)", body, tt.want, errs)
			}
		})
	}
}

// TestRecoverExpr checks that Recover keeps a property or function whose
// expression after '=' is broken, with an ErrorExpr.
func TestRecoverExpr(t *testing.T) {
	src := "val p = foo(1 2)\n\nfun g() = 1 + )\n\nval q: Int get() = foo(1 2)\n\nfun b() = 2\n"
	s := psi.Parse("a.kt", src)
	f, _, errs := Recover(s)
	if len(errs) < 3 {
		t.Errorf("errors %v, want 3 or more", errs)
	}
	var got []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			got = append(got, d.Name+"="+exprKind(d.ExprBody))
		case *syntax.PropDecl:
			if d.Getter != nil {
				got = append(got, d.Name+".get="+exprKind(d.Getter.ExprBody))
			} else {
				got = append(got, d.Name+"="+exprKind(d.Init))
			}
		}
	}
	if want := "p=error g=error q.get=error b=int"; strings.Join(got, " ") != want {
		t.Errorf("got %q, want %q", strings.Join(got, " "), want)
	}
}

// summarize describes the statements of fd's body.
func summarize(fd *syntax.FunDecl) string {
	if fd.Body == nil {
		return exprKind(fd.ExprBody)
	}
	var out []string
	for _, s := range fd.Body.Stmts {
		switch s := s.(type) {
		case *syntax.VarStmt:
			if _, ok := s.Init.(*syntax.ErrorExpr); ok {
				out = append(out, "var "+s.Name+"=error")
			} else {
				out = append(out, "var "+s.Name)
			}
		case *syntax.ExprStmt:
			out = append(out, exprKind(s.X))
		default:
			out = append(out, fmt.Sprintf("%T", s))
		}
	}
	return strings.Join(out, "; ")
}

func exprKind(x syntax.Expr) string {
	switch x.(type) {
	case *syntax.ErrorExpr:
		return "error"
	case *syntax.Call:
		return "call"
	case *syntax.If:
		return "if"
	case *syntax.IntLit:
		return "int"
	}
	return fmt.Sprintf("%T", x)
}
