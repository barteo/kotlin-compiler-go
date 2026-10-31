// Inference that checking kotlinpoet needed: generic arguments of
// overloaded calls, if and when branches inferred together, capture of
// projections, bounds that relate type parameters, the implicit receiver
// and companion that take the arguments, inline-only extension
// properties, and an exhaustive when as a lambda's value.

import kotlin.reflect.KVariance

enum class Mod { A, B, C }

fun pick(b: Boolean, n: Int): List<String> {
    val x = if (b) {
        if (n > 0) listOf("a$n") else listOf("b")
    } else {
        listOf()
    }
    return x
}

fun mods(m: Set<Mod>): Set<Mod> = setOf(Mod.A) + when {
    Mod.B in m -> emptySet()
    Mod.C in m -> setOf(Mod.C)
    else -> emptySet()
}

class Spec(val name: String, val tags: List<String>) {
    constructor(name: String) : this(name, listOf())

    fun copy(name: String = this.name, tags: List<String> = this.tags) = Spec(name, tags)

    fun copy(tags: Map<String, String>) = Spec(name, tags.keys.toList())

    override fun toString() = "$name$tags"
}

fun addAll(set: MutableSet<Mod>, vararg ms: Mod) {
    set += ms
}

open class Base {
    companion object {
        fun get(x: Int) = "base $x"
    }
}

class Kid : Base() {
    companion object {
        fun get(s: String) = "kid $s"

        fun both(n: Int) = get(n) + "/" + get("s")
    }
}

class Writer {
    val out = StringBuilder()

    fun emit(s: String) {
        out.append(s)
    }
}

fun build(block: Writer.() -> Unit): String {
    val w = Writer()
    w.block()
    return w.out.toString()
}

class Node(val name: String) {
    fun emit(w: Writer) = w.emit("<$name>")

    override fun toString() = build {
        emit("[")
        emit(this)
        emit("]")
    }
}

open class TypeName

class Wildcard : TypeName()

val STAR = Wildcard()

fun variances(xs: List<KVariance?>): List<TypeName> = xs.map { v ->
    when (v) {
        null -> STAR
        KVariance.INVARIANT -> TypeName()
        KVariance.IN -> STAR
        KVariance.OUT -> STAR
    }
}

// s === SL0 doesn't make s a Slice<A>: bar(s) still returns X?.
interface Slice<V>
interface Letter
val SL0: Slice<Letter> = object : Slice<Letter> {}

fun <X> sliced(s: Slice<X>): X? {
    if (s === SL0) {
        return unslice(s)
    }
    return null
}

fun <Y> unslice(w: Slice<Y>): Y? = null

// it is an R, R <: S: S isn't fixed first, as Nothing from the return.
interface Some {
    fun method(): String
}

fun <S> either(nullable: S?, notNullable: S): S = nullable ?: notNullable

fun <R : Some> Some.withPredicate(predicate: (R) -> Unit): R? = null

fun elvised(derived: Some): Some {
    return either(derived.withPredicate { it.method() }, return derived)
}

// VR <: T: T is fixed first, from x, since VR has no other bound of
// its own.
fun <T, VR : T> withPrev(x: T, fn: (VR?, T) -> Unit) = fn(null, x)

fun main() {
    println(pick(true, 1) + pick(true, 0) + pick(false, 0))
    withPrev(7) { prev: Int?, new -> println("$prev $new") }
    println(sliced(SL0))
    println(elvised(object : Some { override fun method() = "m"; override fun toString() = "some" }))
    println(mods(setOf(Mod.B)) + mods(setOf(Mod.C)) + mods(emptySet()))
    println(Spec("s").copy(tags = emptyList()))
    println(maxOf(3, 7))
    val s = mutableSetOf<Mod>()
    addAll(s, Mod.B, Mod.C)
    println(s)
    println(Kid.both(3))
    println(listOf("a" to 1, "b" to 2).onEach { (k, v) -> print("$k$v ") }.size)
    println(String::class.java.kotlin.simpleName)
    println(Mod.C.declaringJavaClass.simpleName)
    println(Node("n"))
    println(variances(listOf(null, KVariance.INVARIANT)).map { it is Wildcard })
}
