fun test(a: Unknown) { // ERROR unresolved
    a.toString()
    a.hashCode()
    a.foo() // ERROR unresolved reference: foo
}
