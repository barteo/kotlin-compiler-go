// An intersection with a built-in type has that type's members, but no
// name none of its types has, as with kotlinc.
interface A { fun aaa() }

fun f(x: Any?) {
    if (x is A && x is String) {
        x.length
        x.foo() // ERROR unresolved reference: foo
    }
    if (x is Int && x is String) {
        x.bar // ERROR unresolved reference: bar
    }
}
