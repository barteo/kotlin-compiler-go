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

// A captured val can't be initialized in a local class or in an object's
// functions and accessors, which may run more than once; an object's init
// block runs in place.
fun capturedInLocalMembers() {
    val x: Int
    val y: Int
    val z: Int
    val w: Int
    val v: Int
    object {
        init {
            x = 0
        }

        fun f() {
            y = 0 // ERROR captured values cannot be initialized because of possible reassignments
        }

        val p: Int
            get() {
                z = 0 // ERROR captured values cannot be initialized because of possible reassignments
                return 1
            }
    }
    println(x)
    class Local {
        init {
            w = 0 // ERROR captured values cannot be initialized because of possible reassignments
        }

        fun g(): Int {
            val own: Int
            own = 1
            return own + x
        }
    }
    v = 1
    println(v + Local().g())
}

// An exception in a try's body may come after an assignment on a path
// that jumps out of it: the catch block may assign again.
fun tryJumps(flag: Boolean) {
    val n: Int
    try {
        if (flag) {
            n = 1
            if (flag) throw Exception()
            return
        }
    } catch (e: Exception) {
        n = 3 // ERROR val cannot be reassigned
    }
}

// As kotlinc, this smart cast to a subclass that sees a property with
// another type makes its assignment one of the intersection override, a
// val that the property's initialization doesn't count: y isn't
// initialized. Without a type that differs, it initializes it.
open class CastBase<T> {
    val x: Any?
    val y: T // ERROR property must be initialized or be abstract
    init {
        this as CastDerived
        x = "O"
        this.y = "O" // ERROR val cannot be reassigned
    }
}
class CastDerived : CastBase<String>()

open class CastBase2<T> {
    val y: T // ERROR property must be initialized or be abstract
    init {
        this as CastDerived2
        y = "O" as T // ERROR val cannot be reassigned
    }
}
class CastDerived2 : CastBase2<String>()

open class CastBase3 {
    val y: String
    init {
        this as CastDerived3
        this.y = "O"
    }
}
class CastDerived3 : CastBase3()
