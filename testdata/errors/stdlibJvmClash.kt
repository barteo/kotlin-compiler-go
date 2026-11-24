// Members with bodies of an interface that clash clash again in its
// DefaultImpls class, which kotlinc reports at the interface.

interface T { // ERROR platform declaration clash
    val x: Int
        get() = 1 // ERROR platform declaration clash
    fun getX() = 1 // ERROR platform declaration clash
}

interface U {
    val y: Int // ERROR platform declaration clash
    fun getY(): Int // ERROR platform declaration clash
}

// Constructors that clash are reported at each of them.
@JvmInline
value class A(val x: Int) { // ERROR platform declaration clash
    constructor(x: UInt) : this(x.toInt()) // ERROR platform declaration clash
}
