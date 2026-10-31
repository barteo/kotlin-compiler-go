inner class Top // ERROR modifier 'inner' is not applicable to 'top level class'

object O {
    inner class InObject // ERROR modifier 'inner' is not applicable inside an object or an interface
}

class Outer {
    inner class Inner
    class Nested {
        fun make() = Inner() // ERROR constructor of inner class Outer.Inner can be called only with receiver of containing class
    }
    class NestedDefault(val i: Any = Inner()) // ERROR constructor of inner class Outer.Inner can be called only with receiver of containing class
}

class G<T> {
    inner class I
}

fun f(i: G<Int>.I, j: G.I) {} // ERROR one type argument expected for class G

open class Outer2 {
    open inner class Inner
    class Nested : Inner() // ERROR constructor of inner class Outer2.Inner can be called only with receiver of containing class
    class Nested2 : Inner {
        constructor() // ERROR constructor of inner class Outer2.Inner can be called only with receiver of containing class
    }
}

fun outside() = object : Outer2.Inner() {} // ERROR constructor of inner class Outer2.Inner can be called only with receiver of containing class
