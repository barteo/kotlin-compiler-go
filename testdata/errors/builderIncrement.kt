// x++ for a var x of a builder's variable that an assignment bounds finds
// no inc, as in K2, which doesn't fix the variable for an increment
// (KT-71778), though its type, inferred later, has one.
interface Foo<T : Any> {
    var a: T
}

fun <T : Any> myBuilder(block: Foo<T>.() -> Unit): Foo<T> = TODO()

fun main() {
    myBuilder {
        a = 45
        a++ // ERROR none of the candidates for 'inc'
        a-- // ERROR none of the candidates for 'dec'
        a = 1
    }
}
