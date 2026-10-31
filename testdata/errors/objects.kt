class Outer(val x: Int) {
    fun f(): Int = x

    class Nested {
        fun g(): Int = x // ERROR unresolved reference: x
        fun h(): Int = f() // ERROR unresolved reference: f
    }

    companion object {
        fun k(): Int = x // ERROR unresolved reference: x
    }

    companion object Second // ERROR only one companion object is allowed
}

object O {
    companion object // ERROR a companion object is not allowed in an object
}

object P(val y: Int) // ERROR constructors are not allowed for objects

open object Q // ERROR modifiers open and abstract are not allowed on objects

companion object Top // ERROR modifier 'companion' is only allowed on objects in classes

class Plain

fun make() {
    val o = O() // ERROR expression 'O' of type 'O' cannot be invoked as a function
    val c = Plain // ERROR class 'Plain' cannot be used as a value
    val d = Outer
}
