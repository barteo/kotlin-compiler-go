// A class's type parameters' bounds don't see its own nested classes,
// inner classes or companion, nor those it inherits; an enclosing class's
// are in scope.
class A<T : Nested> { class Nested } // ERROR unresolved or unsupported type: Nested
class B<T> where T : Inner { inner class Inner } // ERROR unresolved or unsupported type: Inner
class C<T : Companion> { companion object } // ERROR unresolved or unsupported type: Companion
open class Base { class BaseNested }
class D<T : BaseNested> : Base() // ERROR unresolved or unsupported type: BaseNested

class Qualified<T : Qualified.Nested> { class Nested }
class Outer<X> {
    class N
    class S<T : N>
    inner class I<T : X>
    inner class J<T : N>
    interface Face<T : N>
}
class Recursive<T : Recursive<T>>
