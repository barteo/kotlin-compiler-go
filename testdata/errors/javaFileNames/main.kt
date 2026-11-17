// Kotlin sees a Java source's top-level classes only if they are named as
// their files, as kotlinc finds them by the files' names (KT-4455).
fun main(x: A) {
    x.ok()
    x.b() // ERROR B
    D().baz() // ERROR unresolved reference
}
