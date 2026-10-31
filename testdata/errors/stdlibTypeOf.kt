import kotlin.reflect.typeOf

class Holder<H>

fun <R : Comparable<R>> recursive() = typeOf<Holder<R>>() // ERROR non-reified type parameters with recursive bounds are not supported yet: R

fun <A, B : List<A>> chained() where A : Comparable<B> = typeOf<Holder<B>>() // ERROR non-reified type parameters with recursive bounds are not supported yet: A

fun <S : Comparable<String>> bounded() = typeOf<Holder<S>>()

inline fun <reified R : Comparable<R>> reifiedRecursive() = typeOf<R>()
