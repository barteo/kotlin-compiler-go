// A variable that only has a declared bound is inferred from it when
// the bound is a self type, as K2 does: a class with a type parameter
// bounded by the class itself, or a type parameter bounded by one.

open class Builder<B : Builder<B>> {
    var name = "builder"
    @Suppress("UNCHECKED_CAST")
    fun <T : B> self(): T = this as T
    fun named(n: String): B {
        name = n
        @Suppress("UNCHECKED_CAST")
        return this as B
    }
}

class Leaf : Builder<Leaf>()

// T : B, with B a star: the bound Builder<*>.
fun star(b: Builder<*>): String = b.self().named("star").name

// T : K, with K : Builder<K>: K.
fun <K : Builder<K>> param(b: Builder<K>): K = b.self()

// B of <B : Builder<B>> with nothing else: Kotlin's captured star.
@Suppress("UNCHECKED_CAST")
fun <B : Builder<B>> fresh(): B = Leaf() as B

class Klass<T : Klass<T>> {
    @Suppress("UNCHECKED_CAST")
    fun me(): T = this as T
}

fun <T : Klass<T>> Klass<T>.ext(): String = "ext " + (me() === this)

// G is the receiver's captured star, and T below it.
fun <G : Enum<G>, T : G> Enum<G>.same(): T {
    @Suppress("UNCHECKED_CAST")
    return this as T
}

enum class Color { RED, GREEN }

fun entry(e: Enum<*>): String = e.same().name

fun main() {
    println(star(Leaf()))
    println(param(Leaf()).named("param").name)
    val f = fresh()
    val b: Builder<*> = f
    println(b.named("fresh").name)
    println(Klass().ext())
    val k = Klass()
    val k2: Klass<*> = k.me()
    println(k2 === k)
    println(entry(Color.GREEN))
}
