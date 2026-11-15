// Declarations of the root package, which a library's user in another
// package imports by their simple names.
class Klass {
    fun hello() = "klass"
    class Nested {
        fun hello() = "nested"
    }
}

object Objekt {
    fun hello() = "objekt"
}

typealias KlassAlias = Klass
typealias NestedAlias = Klass.Nested
typealias ObjektAlias = Objekt
typealias Greeting = (String) -> String

fun topLevel() = "top"

val topLevelVal = "val"
