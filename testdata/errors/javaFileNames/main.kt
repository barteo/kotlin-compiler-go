// As kotlinc (KT-4455), a Java source's top-level class is found only
// through the file named after it.
fun main(x: A) {
    x.b().bar() // ERROR cannot access class 'B'
    x.f().foobaz() // ERROR cannot access class 'F'
    D().baz() // ERROR unresolved reference
    val g: G? = null // ERROR G
    x.ok()
}
