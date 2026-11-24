// A property assigned in an init block must still be initialized where it
// is declared if its setter has a body, which the assignment would call,
// or if it is open, where an override would be assigned instead.
open class Foo {
    var a00: Int
    var a01: Int; set(v) { field = v } // ERROR property must be initialized
    var a02: Int; set
    var a03: Int; set(v) {} // ERROR property must be initialized
    var a33: Int; set(v) {} get() = 1
    open var b00: Int // ERROR property must be initialized, be final, or be abstract
    open var b10: Int; get() = field // ERROR property must be initialized or be final
    open var b33: Int; set(v) {} get() = 1
    private var p: Int
    final var q: Int

    init {
        a00 = 1; a01 = 1; a02 = 1; a03 = 1
        b00 = 1; b10 = 1; p = 1; q = 1
    }
}

class Bar {
    var a00: Int
    var a01: Int; set(v) { field = v } // ERROR property must be initialized
    var e00: Int // ERROR property must be initialized or be abstract

    init {
        a00 = 1; a01 = 1
    }
}
