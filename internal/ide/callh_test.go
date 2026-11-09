package ide

import (
	"fmt"
	"strings"
	"testing"
)

const callhSrc = `package p

open class Shape {
    open fun area(): Int = 0
}

class Square(val side: Int) : Shape() {
    init { log("square") }
    override fun area() = side * side
    constructor() : this(1)
}

fun log(s: String) = println(s)

val unit = Square(1)

fun total(shapes: List<Shape>): Int {
    var sum = 0
    shapes.forEach { sum += it.area() }
    fun local() = log("local")
    local()
    return sum + Square().area()
}
`

func fmtCalls(cs []CallerCall) string {
	var b strings.Builder
	for _, c := range cs {
		fmt.Fprintf(&b, "%s(%s)", c.Item.Name, c.Item.Detail)
		for _, r := range c.Ranges {
			fmt.Fprintf(&b, " %d", r.Start)
		}
		b.WriteString("; ")
	}
	return b.String()
}

func TestCallHierarchy(t *testing.T) {
	v := testView(t, "a.kt", callhSrc)
	at := func(s string) int { return strings.Index(callhSrc, s) }
	item, ok := PrepareCallHierarchy(v, "a.kt", at("area()")+1)
	if !ok || item.Name != "area" || item.Kind != MethodSymbol || item.Detail != "Shape" {
		t.Errorf("prepare: %+v %v", item, ok)
	}
	if item, ok := PrepareCallHierarchy(v, "a.kt", at("Square(1)")); !ok || item.Name != "Square" {
		t.Errorf("prepare on a constructor call: %+v %v", item, ok)
	}
	for _, tc := range []struct {
		decl, want string
	}{
		// area's override family, called from total through Shape and
		// Square.
		{"area()", fmt.Sprintf("total(p) %d %d; ", at("area() }"), at("area()\n}"))},
		// From the init block, as its class's; and from a local function.
		{"log(s", fmt.Sprintf("Square(p) %d; local(total) %d; ", at("log(\"square"), at("log(\"local"))},
		// A class's constructor calls, from a property's initializer and
		// through its secondary constructor.
		{"Square(val", fmt.Sprintf("unit(p) %d; total(p) %d; ", at("Square(1)"), at("Square()."))},
	} {
		if got := fmtCalls(IncomingCalls([]*View{v}, "a.kt", at(tc.decl))); got != tc.want {
			t.Errorf("incoming %s: %s, want %s", tc.decl, got, tc.want)
		}
	}
	want := fmt.Sprintf("area(Shape) %d; area(Square) %d; constructor(Square) %d; local(total) %d; ", at("area() }"), at("area()\n}"), at("Square()."), at("local()\n"))
	if got := fmtCalls(OutgoingCalls(v, "a.kt", at("total("))); got != want {
		t.Errorf("outgoing total: %s, want %s", got, want)
	}
	if got := fmtCalls(OutgoingCalls(v, "a.kt", at("Square(val"))); got != fmt.Sprintf("log(p) %d; ", at("log(\"square")) {
		t.Errorf("outgoing Square: %s", got)
	}
}
