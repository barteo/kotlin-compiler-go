interface Test() { // ERROR an interface may not have a constructor
    init { // ERROR anonymous initializers are not allowed in interfaces
    }

    private fun foo(): Int // ERROR private function 'foo' in an interface must have a body
    private fun bar(): Int = 1
    fun baz(): Int
}

interface Ok {
    @get:JvmStatic // ERROR only members in named objects and companion objects can be annotated with '@JvmStatic'
    val a: Int
}

class C {
    var p: Int = 1
        @JvmStatic set(v) { // ERROR only members in named objects and companion objects can be annotated with '@JvmStatic'
            field = v
        }

    companion object {
        var q: Int = 1
            @JvmStatic set(v) {
                field = v
            }
    }
}
