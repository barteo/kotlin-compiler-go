interface Greeter {
    fun greet(name: String): String
}

abstract class Base(val n: Int)

inline fun <reified T> twice(f: () -> Unit) {
    f()
    f()
    val o = object : Greeter {
        override fun greet(name: String) = object { // ERROR object expressions in object expressions in inline functions with reified type parameters are not supported
            override fun toString() = name
        }.toString()
    }
}

fun main() {
    val a = object : Greeter {} // ERROR object is not abstract and does not implement abstract member 'greet'
    val b = object : Greeter {
        override fun greet(name: String) = this@Missing.toString() // ERROR unresolved label: Missing
    }
    val c = object : Base() {} // ERROR no function '<init>' takes 0 arguments
    val d = object {
        constructor() // ERROR constructors are not allowed for objects
    }
    while (true) {
        val e = object : Runnable {
            override fun run() {
                break // ERROR 'break' is not allowed outside a loop
            }
        }
    }
}
