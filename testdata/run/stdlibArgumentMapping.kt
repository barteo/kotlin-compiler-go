// Calls whose arguments go to parameters other than those at their
// positions, or that resolve past the first functions they find.

var log = ""
fun <T> logged(v: T): T {
    log += "$v;"
    return v
}

// A class that inherits f from a superclass and from an interface with
// default values: its intersection override takes them.
interface I<T> {
    fun f(x: String = "i"): String
    fun g(x: T): Any
}
interface J<T> : I<T> {
    override fun f(x: String): String
}
open class A<T> {
    open fun f(x: String) = "A.f($x)"
    fun g(x: T): String = "A.g($x)"
}
class B : A<String>(), I<String>
class C : A<Int>(), J<Int>

interface Named {
    fun name(prefix: String = "<", suffix: String = ">"): Any
}
open class Base {
    fun name(prefix: String, suffix: String): String = prefix + "base" + suffix
}
class Sub : Base(), Named

interface Greeter {
    fun greet(name: String): String = "hi $name"
}
interface Defaulted {
    fun greet(name: String = "you"): String
}
class Delegating(g: Greeter) : Greeter by g, Defaulted

// Indexed assignments pass the value to set's last parameter.
class Table {
    var log = ""
    operator fun get(row: Int, col: Int = 0) = row * 10 + col
    operator fun set(row: Int, col: Int = 0, value: Int) {
        log += "[$row,$col]=$value "
    }
}
object Grid {
    var sum = 0
    operator fun get(vararg ix: Int) = sum
    operator fun set(vararg ix: Int, value: Int) {
        sum = ix.sum() + value
    }
}

fun joined(vararg xs: Int, sep: String, end: String = ".") = xs.joinToString(sep) + end
fun middle(a: Int, vararg xs: String, b: Double) = "$a ${xs.joinToString()} $b"

// A lambda after an expression in parentheses calls its value's invoke.
operator fun <K, V> Pair<K, V>.invoke(f: (K, V) -> String) = f(first, second)

interface Factory
operator fun Factory.invoke(n: Int) = "made $n"
class Made {
    companion object : Factory
}

fun locals(): String {
    var s = ""
    fun add(x: String, n: Int) {
        s += x.repeat(n)
    }
    run {
        fun add(x: String) {
            s += x
        }
        add("a", 2)
        add("b")
    }
    return s
}

fun main() {
    val b = B()
    println(b.f() + " " + b.f("x") + " " + b.g("y"))
    val i: I<String> = b
    println(i.f())
    println(C().f())
    println(Sub().name() + Sub().name(suffix = "]") + Sub().name("(", ")"))
    println(Delegating(object : Greeter {}).greet())

    val t = Table()
    t[1] = 5
    t[2, 3] = 6
    t[4] += 1
    println(t.log)
    Grid[1, 2] = 3
    println(Grid.sum)
    Grid[1, 2, 3] += 4
    println(Grid.sum)

    println(joined(1, 2, sep = "-"))
    println(joined(xs = *intArrayOf(3, 4), "+", end = "!"))
    println(middle(1, xs = *arrayOf("p", "q"), 2.0))
    println(middle(a = 1, "r", "s", b = 3.0))
    log = ""
    joined(logged(1), logged(2), end = logged("e"), sep = logged("s"))
    println(log)

    println((1 to "one") { k, v -> "$k=$v" })
    println(Made(7))
    println(locals())
}
