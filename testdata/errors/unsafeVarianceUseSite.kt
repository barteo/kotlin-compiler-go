// @UnsafeVariance matters only for declaration-site variance: through a
// use-site projection of an invariant type parameter, a parameter of type
// @UnsafeVariance T takes Nothing, as any parameter of type T does.

interface A<T> {
    fun foo(x: @UnsafeVariance T)
}

fun useSite(a1: A<out Any?>, a2: A<*>) {
    a1.foo("") // ERROR type mismatch
    a2.foo("") // ERROR type mismatch
}

interface Source<out T> {
    fun has(x: @UnsafeVariance T): Boolean
}

fun declarationSite(s: Source<*>, t: Source<CharSequence>) = s.has("") && t.has("")
