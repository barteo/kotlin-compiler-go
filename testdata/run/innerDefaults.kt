// Default values of the methods and constructors of inner classes that use
// the outer instances, this@Outer, and an outer class's members.

interface Owner

class A {
    override fun toString(): String = "A"

    inner class B {
        override fun toString(): String = "B"

        inner class C(val a: A = this@A, val b: B = this@B) {
            override fun toString(): String = "C$a$b"
            fun f(aa: A = this@A, bb: B = this@B): String = "$aa$bb"
        }
    }
}

class Something<T : Owner>(private val ref: T) {
    val Owner.offset
        get() = 42

    private val base = 100

    private inner class Item {
        fun update(owner: T, offset: Int = owner.offset, extra: Int = base): Int = offset + extra
    }

    fun ping(): Int = Item().update(ref) + Item().update(ref, 1)
}

fun main() {
    val c = A().B().C()
    println(c)
    println(c.f())
    println(Something(object : Owner {}).ping())
}
