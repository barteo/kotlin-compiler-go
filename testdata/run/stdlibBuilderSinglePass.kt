// A builder's lambda is checked once, with its variables open, and their
// values are written into what the check gave it: the types of its
// locals, and of the local functions, classes and objects it declares,
// some of which their inferred types give the variables.

class Box<T>(var value: T)

fun <T> collect(block: MutableList<T>.() -> Unit): List<T> = mutableListOf<T>().apply(block)

fun main() {
    // A local function whose inferred result is the builder's variable.
    println(buildList {
        add("a")
        fun twice() = this + this
        addAll(twice())
    })
    // An object whose member's inferred type is the builder's.
    val xs = collect {
        add(1L)
        val src = object {
            fun produce() = this@collect
        }
        src.produce().add(2L)
    }
    println(xs.sum())
    // A local class over the builder's variable.
    println(buildList {
        class Holder(val list: MutableList<Int>) {
            fun push(x: Int) = list.add(x)
        }
        Holder(this).push(3)
        add(4)
    })
    // Locals of the variable's types, boxed and read back.
    println(buildMap {
        put("k", 1.5)
        val b = Box(this)
        b.value["m"] = 2.5
        println(b.value.size)
    })
    // A sequence that yields from a nested builder.
    println(sequence {
        yield(buildList { add('x'); add('y') })
        yieldAll(listOf(listOf('z')))
    }.toList())
}
