class Scope(val name: String)

fun Scope.run(block: Scope.() -> Unit) = block()

fun runNoInline(s: Scope, block: Scope.() -> Unit) = s.block()

class Holder {
    private lateinit var value: String

    fun check(): String {
        var out = ""
        Scope("s").run {
            out += if (!(::value.isInitialized)) "unset $name" else "set $value"
        }
        runNoInline(Scope("t")) {
            out += if (::value.isInitialized) " set $value" else " unset $name"
        }
        return out
    }

    fun set(v: String) {
        value = v
    }
}

fun main() {
    val h = Holder()
    println(h.check())
    h.set("x")
    println(h.check())
}
