class A(b: Boolean) {
    val x: Int
    val y: String
    var z: Int // ERROR property must be initialized or be abstract

    init {
        if (b) x = 1 else x = 2
        println(y) // ERROR variable 'y' must be initialized
        y = "a"
        y = "b" // ERROR val cannot be reassigned
    }

    fun later() {
        z = 1
    }
}

class B {
    val p: Int // ERROR property must be initialized or be abstract

    constructor(v: Int) {
        p = v
    }

    constructor() {
    }
}

class C {
    val q: Int = 1

    fun set() {
        q = 2 // ERROR val cannot be reassigned
    }
}

fun f(c: Boolean) {
    val a: Int
    println(a) // ERROR variable 'a' must be initialized
    val b: Int
    if (c) b = 1
    println(b) // ERROR variable 'b' must be initialized
    val d: Int
    while (c) {
        d = 1 // ERROR val cannot be reassigned
    }
    val e: Int
    val g = { e = 1 } // ERROR captured values cannot be initialized because of possible reassignments
    var h: String
    for (i in 1..2) {
        h = "x"
    }
    println(h) // ERROR variable 'h' must be initialized
}
