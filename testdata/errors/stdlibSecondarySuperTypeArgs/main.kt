// A secondary constructor's super(...) call passes its arguments to the
// superclass's constructor with the supertype's type arguments: a T1 to
// a Y of B<T1, T2> must be a T2, and isn't inferred to be one.
open class B<X, Y : X> {
    constructor(x: X, y: Y)
    constructor(x: X, s: String)
    constructor(y: Y, i: Int) : this(y, "")
}

class A<T1, T2 : T1> : B<T1, T2> {
    constructor(x: T1, y: T2) : super(x, y)
    constructor(x: T2, y: T2, z: String) : super(x, y)
    constructor(x: T2, z: String, z1: String) : super(x, "")
    constructor(x: T2, z: String, z1: String, z2: String) : super(x, 1)
    constructor(x: T1, z: String, z1: String, z2: String, z3: String) : super(x, "")
    constructor(x: T1, z: String, z1: String, z2: String, z3: String, z4: String) : super(x, 1) // ERROR none of the 3 candidates
}

class S : B<CharSequence, String> {
    constructor(s: String) : super(s, 1)
    constructor(c: CharSequence, i: Int) : super(c, 1) // ERROR none of the 3 candidates
    constructor(c: CharSequence) : super(c, "")
}

open class L<T>(val l: List<T>)
class LS : L<CharSequence> {
    constructor() : super(listOf("a")) // List<String> is a List<out CharSequence>
    constructor(i: Int) : super(emptyList())
    constructor(s: String) : super(mutableListOf(s))
}

typealias BS = B<Any, String>
class TA : BS {
    constructor(s: String) : super(s, 1)
    constructor(a: Any) : super(a, 1) // ERROR none of the 3 candidates
}

class JS : J<CharSequence> {
    constructor(s: String) : super(s, s)
    constructor(c: CharSequence) : super(c, 1) // ERROR CharSequence
    constructor(l: List<String>) : super(l)
}

open class O<T> {
    open inner class I<U>(t: T, u: U)
    inner class K : I<Int> {
        constructor(t: T) : super(t, 1)
        constructor(s: String) : super(s, 1) // ERROR type mismatch
    }
}
