import kotlin.properties.Delegates

// Declarations that kotlinc doesn't see as conflicting.

fun locals(): String {
    class Local
    fun Local.bar() = "O"
    class Local2<T>
    fun Local2<String>.bar() = "K"
    fun Int.bar() = this + 1
    return Local().bar() + Local2<String>().bar() + 41.bar()
}

class Hidden {
    val a = 1
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    val a = ""

    fun f(x: Int) = x * 2
    @Deprecated("", level = DeprecationLevel.HIDDEN)
    fun f(x: Int) {}
}

class Delegated {
    val `x$delegate`: String = "field"
    var x: String by Delegates.notNull()
}

fun main() {
    println(locals())
    println(Hidden().f(21))
    val d = Delegated()
    d.x = "delegated"
    println(d.x + " " + d.`x$delegate`)
}
