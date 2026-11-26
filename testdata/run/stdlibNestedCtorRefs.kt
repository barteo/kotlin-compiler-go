// A reference to a nested class's constructor through an object, a
// companion or a typealias of its outer class, as K2 resolves it.
object A {
    class Wrapper { override fun toString() = "A.Wrapper" }
    class Named(val n: String) { override fun toString() = "Named($n)" }
}
class Outer {
    companion object {
        class Wrapper { override fun toString() = "Outer.Companion.Wrapper" }
    }
    class Plain(val i: Int) { override fun toString() = "Plain($i)" }
}
typealias OuterAlias = Outer
enum class E { X; class Nested { override fun toString() = "E.Nested" } }
typealias EA = E
fun main() {
    println(A::Wrapper.invoke())
    println((A)::Wrapper.invoke())
    println(listOf("a", "b").map(A::Named))
    println(Outer.Companion::Wrapper.invoke())
    println(listOf(1, 2).map(OuterAlias::Plain))
    println(EA::Nested.invoke())
    println(Outer::Plain.invoke(3))
}
