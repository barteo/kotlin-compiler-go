// An inner class's constructor called through its outer class's name
// takes no implicit outer instance, even in the outer class's code.

open class Outer {
    inner class I
    fun f() = Outer.I() // ERROR can be called only with receiver of containing class
    fun g() = I()
    fun h() = this.I()
}

class Sub : Outer() {
    fun k() = Outer.I() // ERROR can be called only with receiver of containing class
    fun m() = I()
}
