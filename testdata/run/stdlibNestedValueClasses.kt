// Value classes whose value is another value class's: on the JVM, the
// innermost value.

@JvmInline value class Id(val v: Int) {
    fun next() = Id(v + 1)
}

@JvmInline value class UserId(val id: Id) {
    fun bump() = UserId(id.next())
}

@JvmInline value class Name(val s: String)

@JvmInline value class Wrapped(val n: Name) {
    override fun toString() = "W(" + n.s + ")"
}

@JvmInline value class Maybe(val id: Id?)

@JvmInline value class Text(val s: String?)

@JvmInline value class Doc(val t: Text)

@JvmInline value class Defaulted(val n: Name = Name("default"))

interface Shown {
    fun show(): String
}

@JvmInline value class Deep(val u: UserId) : Shown {
    override fun show() = "deep " + u.id.v
}

fun take(u: UserId): Int = u.id.v

fun doc(empty: Boolean): Doc? = if (empty) null else Doc(Text(null))

fun eq(a: Any?, b: Any?) = a == b

fun main() {
    val u = UserId(Id(41)).bump()
    println(take(u))
    println(u)
    val any: Any = u
    println(any)
    println((any as UserId).id.v)
    println(Wrapped(Name("x")))
    println(Maybe(null).id)
    println(Maybe(Id(3)).id?.v)
    println(listOf(UserId(Id(1)), UserId(Id(2))).map { it.id.v }.sum())
    val shown: Shown = Deep(UserId(Id(9)))
    println(shown.show())
    println(u == UserId(Id(42)))
    println(u.hashCode())
    println(eq(doc(true), doc(false)))
    println(doc(false))
    println(Defaulted())
    println(Defaulted(Name("given")).n.s)
}
