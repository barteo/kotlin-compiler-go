open class Base(val x: Int)

class A(val y: Int) {
    constructor(s: String) { // ERROR primary constructor call expected
    }

    constructor(d: Double) : this(d.toInt())

    constructor(l: Long) : this("x", 1) // ERROR no function '<init>' takes 2 arguments
}

class B : Base(1) { // ERROR supertype initialization is impossible without a primary constructor
    constructor(x: Int) : super(x)
}

class C : Base {
    constructor(x: Int) : super(x)
    constructor() // ERROR explicit 'this' or 'super' call is required
}

class D {
    constructor(x: Int) : this(x.toString()) // ERROR there's a cycle in the delegation calls chain
    constructor(s: String) : this(s.length) // ERROR there's a cycle in the delegation calls chain
}

class E {
    constructor(x: Int) // ERROR conflicting overloads
    constructor(y: Int) // ERROR conflicting overloads
}

object O {
    constructor() // ERROR constructors are not allowed for objects
}

interface I {
    constructor(x: Int) // ERROR an interface may not have a constructor
}

data class F(val x: Int) {
    constructor() : this(0)
}

data class G { // ERROR data class must have at least one primary constructor parameter
    constructor(x: Int)
}

enum class H {
    X, Y(1);
    constructor()
    constructor(x: Int) : super() // ERROR call to super is not allowed in enum constructor
}

enum class H2(val x: Int) {
    X;
    constructor() : super() // ERROR primary constructor call expected
}

enum class H3 internal constructor(val x: Int) { // ERROR constructor must be private in enum class
    X;
    public constructor() : this(1) // ERROR constructor must be private in enum class
    private constructor(s: String) : this(s.length)
}

class J(val v: Int) {
    constructor(s: String) : this(s.length + v) // ERROR unresolved reference: v
}

class K {
    val k = 1
    constructor(x: Int) {
        println(k + x + q) // ERROR unresolved reference: q
    }
}

fun main() {
    A(1)
    A("s")
    K("no") // ERROR type mismatch
    C()
}
