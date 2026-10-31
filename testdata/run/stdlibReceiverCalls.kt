// r.(f)(args): a call of an extension function value with an explicit
// receiver, whose function is evaluated before the receiver. Also
// labeled anonymous functions and annotations on receiver types and
// type arguments.

@Retention(AnnotationRetention.RUNTIME)
annotation class Tag(val s: String)

@Target(AnnotationTarget.TYPE)
annotation class TypeTag(val s: String)

var log = ""

fun receiver(): Int {
    log += "receiver "
    return 20
}

fun op(f: Int.(Int) -> Int): Int.(Int) -> Int {
    log += "op "
    return f
}

val twice: String.() -> String = { this + this }

class Box(val n: Int) {
    companion object
}

val describe: Any.() -> String = { "<" + this::class.simpleName + ">" }

fun @receiver:Tag("Str" + "ing") String.shout() = uppercase() + "!"

var @receiver:Tag("var") String.size: Int
    get() = length
    set(v) {}

fun <T> id(x: T) = x

fun main() {
    println(receiver().(op { this + it })(1))
    println(log)
    println("ab".(twice)())
    println("O".(fun String.(y: String): String = this + y)("K"))
    println(3.(fun Int.(): (Int) -> Int = { it * this })()(4))
    println(Box.(describe)())
    println(Box(1).(describe)())
    val names = listOf("x", "y").map { "n".(fun String.(i: Int) = this + i)(it.length) }
    println(names)

    val d = d@ fun(n: Int): String {
        if (n < 0) return@d "negative"
        return "non-negative"
    }
    println(d(-1) + " " + d(1))
    val c = (c@ fun Int.() = this + 1)
    println(5.c())

    println("hi".shout())
    val file = Class.forName("StdlibReceiverCallsKt")
    for ((name, params) in listOf("shout" to arrayOf<Class<*>>(String::class.java), "getSize" to arrayOf<Class<*>>(String::class.java), "setSize" to arrayOf<Class<*>>(String::class.java, Int::class.java))) {
        println(name + " " + file.getMethod(name, *params).parameterAnnotations.map { ps -> ps.map { (it as Tag).s } })
    }
    println(id<@TypeTag("Int" + "Arg") Int>(7))
    val xs: MutableList<out @TypeTag("Out") String> = mutableListOf("a")
    println(xs)
}
