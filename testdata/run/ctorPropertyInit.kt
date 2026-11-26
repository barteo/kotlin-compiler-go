// In property initializers and init blocks, the name of a primary
// constructor parameter that declares a property is the property's: it
// reads the property's current value, through an overriding getter, also
// from lambdas, local functions, anonymous functions and objects created
// there. A parameter that declares no property is read as it is.

class Counter(var count: Int) {
    init {
        count = 3
        println("init $count")
    }
    val doubled = count * 2
    val read: () -> Int = { count }
    private val obj = object {
        fun get() = count
    }
    val anon = fun(): Int = count
    fun objGet() = obj.get()
}

open class Base(open val v: Int) {
    val seen = v
    init {
        println("Base init $v")
    }
}

class Derived(x: Int) : Base(x) {
    override val v: Int get() = 42
}

class Private(private var p: Int) {
    init {
        p += 1
        println("Private $p")
    }
    fun get() = p
}

class Plain(v: Int) {
    var v: Int = v + 1
    val read: () -> Int = { v }
}

class Defaults(val a: Int, val b: Int = a * 10) : Base(a) {
    val sum = a + b
}

fun main() {
    val c = Counter(1)
    c.count = 9
    println("${c.doubled} ${c.read()} ${c.objGet()} ${c.anon()}")
    println(Derived(1).seen)
    println(Private(1).get())
    val p = Plain(1)
    p.v = 7
    println(p.read())
    val d = Defaults(2)
    println("${d.b} ${d.sum}")
}
