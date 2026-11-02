class Outer {
    inner class Inner {
        object Nested // ERROR a class is not allowed here
        interface NI // ERROR a class is not allowed here
        enum class NE { X } // ERROR a class is not allowed here
        companion object // ERROR a class is not allowed here
        inner class Ok
    }
}
enum class E {
    X {
        class InEntry // ERROR a class is not allowed here
        inner class OkInner
    }
}
sealed class S
sealed interface SI
fun f() {
    class L : S() // ERROR a local class cannot extend a sealed class or interface
    val o = object : SI {} // ERROR a local class cannot extend a sealed class or interface
    class LocalOuter {
        class NestedInLocal // ERROR 'Class' is prohibited here
        inner class Ok
    }
    val a = object {
        class InAnon // ERROR 'Class' is prohibited here
    }
}
open class C1 : C2.N() { // ERROR there's a cycle in the inheritance hierarchy for this type
    open class N
}
open class C2 : C1() { // ERROR there's a cycle in the inheritance hierarchy for this type
    open class N
}
