// Class::name binds to the companion object when the class has no member
// of the name: Level::parse.
enum class Level(val value: String) {
    ON("on"), OFF("off");

    companion object {
        fun parse(value: String): Level? = values().firstOrNull { it.value == value }
        val default = OFF
    }
}

inline fun <T, R> T.let2(f: (T) -> R): R = f(this)

fun <T> Array<T>.firstOrNull(p: (T) -> Boolean): T? {
    for (t in this) if (p(t)) return t
    return null
}

class Box(val n: Int) {
    fun show() = "box $n"

    companion object {
        fun make(n: Int) = Box(n)
        fun show() = "companion"
    }
}

fun main() {
    println("on".let2(Level::parse))
    println("x".let2(Level::parse))
    val f: (String) -> Level? = Level::parse
    println(f("off"))
    println(3.let2(Box::make).show())
    // The class's own member wins: an unbound reference.
    val s: (Box) -> String = Box::show
    println(s(Box(1)))
}
