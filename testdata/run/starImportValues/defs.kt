package defs

enum class E {
    ENTRY, ANOTHER;
    class Nested { companion object { fun foo() = 42 } }
}


object O {
    operator fun invoke() = "O.invoke"
}

typealias AO = O
typealias S = String
