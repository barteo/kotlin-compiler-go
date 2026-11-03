// A local or member extension function comes before an extension-typed
// property of the same name; foo() on an implicit receiver calls the val.

class A {
    fun test1(): Boolean {
        val foo: String.() -> Boolean = { false }
        fun String.foo(): Boolean = true
        return "1".foo()
    }
    fun test2(): Boolean {
        val foo: String.() -> Boolean = { false }
        fun String.foo(): Boolean = true
        with("2") { return foo() }
    }
}

class B {
    val foo: String.() -> Boolean = { false }
    fun String.foo(): Boolean = true
    fun test3() = "1".foo()
    fun test4(): Boolean { with("2") { return foo() } }
}

fun main() {
    val bar: Int.() -> Int = { this + 1 }
    println(A().test1())
    println(A().test2())
    println(B().test3())
    println(B().test4())
    println(5.bar())
}

inline fun <T, R> with(r: T, block: T.() -> R): R = r.block()
