// Programs kc used to reject: more of the box tests' last check errors.

// this in a class's header is its supertype's companion, or the object
// it is nested in.
abstract class Named(val name: String)

open class Base {
    companion object {
        val label = "companion"
    }
}

class Child : Base() {
    class Tag : Named(this.label)
}

object Registry {
    val prefix = "registry"
    class Entry : Named(this.prefix)
}

// An object expression sees the type parameters around it.
class Wrapper {
    private fun <T : Any> T.self() = object {
        fun calc(): T = this@self
    }
    fun twice() = 1.self().calc() + 1
}

// listOf's T, fixed first, infers the second bar's T.
class Inv<T>
fun <T> bar(x: Inv<T>.() -> Unit) = x

fun main() {
    println(Child.Tag().name)
    println(Registry.Entry().name)
    println(Wrapper().twice())
    val fs = listOf(bar<Char> { }, bar { })
    println(fs.size)
}
