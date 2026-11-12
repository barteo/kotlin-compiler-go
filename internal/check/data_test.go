package check

import (
	"context"
	"slices"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestErrorData checks the fix data of the errors that carry one.
func TestErrorData(t *testing.T) {
	for _, tc := range []struct {
		src, code string
		data      []string
	}{
		{`package p
enum class Color { RED, GREEN, BLUE }
fun f(c: Color) = when (c) { Color.RED -> 1 }`, "NO_ELSE_IN_WHEN", []string{"p.Color.GREEN", "p.Color.BLUE"}},
		{`package p
sealed class Shape
class Circle : Shape()
object Empty : Shape()
sealed class Poly : Shape()
class Square : Poly()
fun f(s: Shape?) = when (s) { is Circle -> 1 }`, "NO_ELSE_IN_WHEN", []string{"p.Empty", "is p.Square", "null"}},
		{`fun f(b: Boolean) = when (b) { true -> 1 }`, "NO_ELSE_IN_WHEN", []string{"false"}},
		{`package p
interface Source<T> { fun read(n: Int, vararg tags: String): T?
  val size: Int
  var name: String? }
class S : Source<String> {
  override val size = 1
  override var name: String? = null
}`, "ABSTRACT_MEMBER_NOT_IMPLEMENTED", []string{"fun read(n: kotlin.Int, vararg tags: kotlin.String): kotlin.String?"}},
		{`package p
abstract class A { abstract var name: String }
class B : A()`, "ABSTRACT_CLASS_MEMBER_NOT_IMPLEMENTED", []string{"var name: kotlin.String"}},
		{`package p
open class A { open fun f(): String = "" }
class B : A() { override fun f(): Any = "" }`, "RETURN_TYPE_MISMATCH_ON_OVERRIDE", []string{"kotlin.String"}},
		{`fun f(): Int = "s"`, "TYPE_MISMATCH", []string{"kotlin.String", "kotlin.Int"}}, // the message's: actual, expected
	} {
		f, _, lerrs := lower.Recover(psi.Parse("t.kt", tc.src))
		if len(lerrs) > 0 {
			t.Fatalf("%s: %v", tc.code, lerrs)
		}
		errs, err := CheckContext(context.Background(), []*syntax.File{f}, NewLibrary(nil), 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range errs {
			if e.Code == tc.code {
				found = true
				if !slices.Equal(e.Data, tc.data) {
					t.Errorf("%s: data %q, want %q", tc.code, e.Data, tc.data)
				}
			}
		}
		if !found {
			t.Errorf("%s: no such error in %v", tc.code, errs)
		}
	}
}
