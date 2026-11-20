// An init block can't assign a property of its class declared after it,
// as kotlinc's INITIALIZATION_BEFORE_DECLARATION, in an object
// expression too with Kotlin 2.4's
// ForbidInitializationBeforeDeclarationInAnonymous. A lambda or a local
// function or class in the block may run later.
interface I { var i: Int? }
open class Base { var b = 0 }

class A : Base(), I {
    init {
        i = 1 // ERROR variable cannot be initialized before declaration
        this.j = 2 // ERROR variable cannot be initialized before declaration
        b = 3
        val f = { j = 4 }
        fun loc() { j = 5 }
    }
    override var i: Int? = null
    var j: Int = 2
    init {
        j = 6
        k = 7 // ERROR variable cannot be initialized before declaration
    }
    val k: Int
}

fun create(): I = object : I {
    init {
        i = 1 // ERROR variable cannot be initialized before declaration
    }
    override var i: Int? = null
}
