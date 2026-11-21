// Overloads conflict, as in K2, if each is as specific as the other,
// whatever their type parameters' names and bounds, if they have as many.

class Inv<T>
interface UserInterface

fun <A> nullability(arg: A?) {} // ERROR conflicting overloads: nullability
fun <B> nullability(arg: B) {} // ERROR conflicting overloads: nullability
fun <C> nullability(arg: C & Any) {}

fun <T> projection(x: Inv<T>) {} // ERROR conflicting overloads: projection
fun <T> projection(x: Inv<out T>) {} // ERROR conflicting overloads: projection

open class Members {
    open fun <T : Inv<UserInterface>> bounds() {} // ERROR conflicting overloads: bounds
    open fun <T : Inv<*>> bounds() {} // ERROR conflicting overloads: bounds

    // Not as many type parameters: the JVM names tell them apart.
    fun <T> arity() {}
    @JvmName("arity2")
    fun arity() {}
}

// One more specific than the other.
class Two<A, B>

fun <T> one(x: Inv<T>) = x
@JvmName("one2")
fun <T : CharSequence> one(x: Inv<T>) = x

fun <T> two(x: Two<T, T>) = x
@JvmName("two2")
fun <T> two(x: Two<T, String>) = x

// Properties conflict the same way, extension ones too.
interface Ctx
typealias CtxAlias = Ctx
interface Recv

context(c: Ctx) val aliased: Int get() = 1 // ERROR conflicting declarations: aliased
context(c: CtxAlias) val aliased: Int get() = 1 // ERROR conflicting declarations: aliased

val Recv.ext: Int get() = 1 // ERROR conflicting declarations: ext
val Recv.ext: Int get() = 1 // ERROR conflicting declarations: ext

val <T> T.renamed: Int get() = 0 // ERROR conflicting declarations: renamed
val <R> R.renamed: Int get() = 0 // ERROR conflicting declarations: renamed

val <T> T.bounded: Int get() = 0
val <T : Any> T.bounded: String get() = ""

context(c: C & Any) val <C> nonNull: Int get() = 1 // ERROR conflicting declarations: nonNull
context(c: C) val <C : Any> nonNull: Int get() = 1 // ERROR conflicting declarations: nonNull

context(c: Ctx) val hidden: Int get() = 1
@Deprecated("", level = DeprecationLevel.HIDDEN)
context(c: CtxAlias) val hidden: Int get() = 1

// Both of a local class's or an object expression's properties of a name.
val obj = object {
    val y = 1 // ERROR conflicting declarations: y
    val y = 2 // ERROR conflicting declarations: y
}

// Two inherited functions that take the same parameters, but not as many
// type parameters, and an override of one of them.
interface A {
    fun <T> foo()
    fun <T> bar()
}
interface B {
    fun foo()
    fun bar()
}
interface AB : A, B { // ERROR inherits conflicting members 'foo'
    override fun bar() // ERROR conflicting overloads: bar
}
