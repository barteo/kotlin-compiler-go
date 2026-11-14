// Captures kc approximates: a lambda's function type takes an out
// projection's bound in its parameters and result, an elvis's left call
// sees a star of the right side's value as below its bound, and an in
// projection's capture must be within the variable's bounds. A raw
// type's generic methods are erased, unless an elvis made it not raw.

interface Inv<E> {
    fun foo(x: (E) -> E) {}
    fun bar(x: (Inv<E>) -> E) {}
    fun e(): E
}

fun lambdas(x: Inv<out CharSequence>) {
    x.foo { it }
    x.bar { it.e() }
    x.foo { it.length } // ERROR type mismatch
}

fun <X> select(vararg x: X): X = x[0]
fun <E> myE(): Out<E>? = null
interface Out<out T>

fun stars(w: Out<*>) {
    select(w, myE() ?: w)
}

fun <T : Any> nonNull(a: Array<T>): Array<T?> = null!!

fun inCapture(a: Array<in Int>) {
    nonNull(a) // ERROR cannot infer type for type parameter 'T'
}

fun StubBasedPsiElement<*>.raw(): String? {
    if ("".hashCode() == 0) {
        return foo1().bar("")
    }
    val s: String = (StubBasedPsiElement.create() ?: return null).bar("")
    return foo2().bar("") // ERROR type mismatch: inferred type is Any! but String? was expected
}
