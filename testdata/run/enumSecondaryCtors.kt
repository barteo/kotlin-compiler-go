// Secondary constructors of enums: as kotlinc's, they are private and
// take the entry's name and ordinal first, which they pass on to this(...)
// or, without a primary constructor, to Enum's. Entries call them
// directly, entries with bodies through their marker constructors.

enum class D(val prop: Int) {
    X(123) { override fun f() = 1 },
    Y() { override fun f() = prop },
    Z("abc") { override fun f() = prop },
    W(1, 2) { override fun f() = prop },
    V,
    U(4, 5);

    constructor() : this(1)
    constructor(x: String) : this(x.length)
    constructor(x: Int, y: Int = 3) : this(x + y)

    open fun f(): Int = 0
}

enum class N {
    A(0), B, C("x") { override fun toString() = "c" }, E(1, 2, 3);

    val n: Int
    var tag = "t"

    constructor(n: Int) {
        this.n = n
    }
    constructor() : this(7) {
        tag = "empty"
    }
    constructor(s: String) {
        n = s.length
        tag = s
    }
    constructor(vararg xs: Int) : this(xs.size + 3)
}

fun main() {
    for (d in D.values()) println("$d ${d.prop} ${d.f()}")
    for (n in N.values()) println("$n ${n.n} ${n.tag} ${n.ordinal}")
}
