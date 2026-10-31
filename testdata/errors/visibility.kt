// A private constructor is only visible in its class.

class Secret private constructor(val x: Int) {
    fun twin() = Secret(x)

    class Nested {
        fun make() = Secret(1)
    }
}

fun leak() = Secret(1) // ERROR cannot access 'constructor': it is private in 'Secret'
