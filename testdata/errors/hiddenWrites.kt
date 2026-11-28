// A property, a setter or an extension property deprecated with level
// HIDDEN can't be used, as K2 resolves around it: an assignment, as a
// read, is of an unresolved name.
var v3: String
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    get() = ""
    set(value) {}

var v4: String
    get() = ""
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    set(value) {}

@Deprecated("", level = DeprecationLevel.HIDDEN)
var v6: String
    get() = ""
    set(value) {}

class C {
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    var p = 1
    var q: Int
        get() = 1
        @Deprecated("", level = DeprecationLevel.HIDDEN)
        set(value) {}
}

@Deprecated("", level = DeprecationLevel.HIDDEN)
val String.eh: Int get() = 1

@Deprecated("", level = DeprecationLevel.ERROR)
val String.ee: Int get() = 1

var String.ehs: Int
    get() = 1
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    set(value) {}

fun test(c: C) {
    v3 = ""
    v4
    v4 = "" // ERROR unresolved reference
    v6 = "" // ERROR unresolved reference
    c.p = 2 // ERROR unresolved reference
    c.q
    c.q = 2 // ERROR unresolved reference
    "".eh // ERROR unresolved reference
    "".ee // ERROR is deprecated with level ERROR
    "".ehs
    "".ehs = 2 // ERROR unresolved reference
}
