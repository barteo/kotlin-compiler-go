// An enum's entries in its own code come before an outer object's
// property; a nested object named entries before the enum's entries, but
// only as a qualifier, E.entries.toString(); the enum's own members and
// locals before both.
object Shadowing {
    val entries = "outer"

    enum class E0 {
        E;
        fun test() = entries
    }
}

enum class E01 {;
    object entries {
        override fun toString() = "nested object"
    }
}

enum class E02(val entries: String) {
    E("constructor property");
    fun test() = entries
}

enum class E04 {
    E;
    val entries = "member"
    fun test() = entries
}

interface I05 {
    val entries: String
        get() = "inherited"
}

enum class E05 : I05 {
    E;
    fun test() = entries
}

enum class E06 {
    A, B;
    companion object {
        val entries = "companion"
        fun names() = E06.entries.joinToString()
    }
    fun local(): String {
        val entries = "local"
        return entries
    }
}

fun main() {
    println(Shadowing.E0.E.test())
    println(E01.entries)            // the enum's entries
    println(E01.entries.toString()) // a qualifier: the nested object
    println(E02.E.test())
    println(E04.E.test())
    println(E05.E.test())
    println(E06.entries)
    println(E06.names())
    println(E06.A.local())
}
