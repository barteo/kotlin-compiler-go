// Unqualified names in a class: its members, then extensions of this,
// then the members of its and its superclasses' companions, then
// top-level declarations, as kotlinc resolves them.

val Any.foo: String get() = "ext on " + this
fun Any.bar(): String = "extfun on " + this
fun baz() = "top baz"
val qux = "top qux"

open class Base {
  companion object {
    fun bar() = "base companion bar"
    override fun toString() = "BaseCompanion"
  }
}

class A : Base() {
  override fun toString() = "A"
  fun f() = foo + " / " + bar() + " / " + baz() + " / " + qux

  companion object {
    val foo = "companion foo"
    fun baz() = "companion baz"
    val qux = "companion qux"
    override fun toString() = "Companion"
  }
}

class B {
  override fun toString() = "B"
  fun f() = foo + " / " + bar()

  companion object {
    override fun toString() = "BCompanion"
  }
}

class C {
  fun f() = qux + " / " + baz()
}

fun main() {
  println(A().f())
  println(B().f())
  println(C().f())
}
