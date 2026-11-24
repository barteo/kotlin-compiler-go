// An enum's entries comes before a top-level entries also in its nested
// classes, unless a class in between has a member entries.
package pckg

val entries = "E"

enum class E {
    X;

    class B {
        fun foo() = entries.length // ERROR unresolved reference: length
        fun bar() = pckg.entries.length
    }

    class C {
        val entries = 0
        fun foo() = entries + 4
    }
}

fun main() {}
