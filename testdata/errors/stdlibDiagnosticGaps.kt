// Single checks that kotlinc 2.4's diagnostics tests expect, each a case
// of a check kc has otherwise.

fun catchNothing() {
    try {
    } catch (e: Nothing) { // ERROR throwable type mismatch: actual type is 'Nothing'
    }
}

fun emptyBranches(c: Boolean) {
    val a = if (c) else false // ERROR 'if' must have both main and 'else' branches if used as an expression
    val b = if (c) true else; // ERROR 'if' must have both main and 'else' branches if used as an expression
    val d = if (c) {} else false
    if (c) else println()
}

annotation class IntArrays(val ok: IntArray, val boxed: Array<Int>) // ERROR invalid type of annotation member

enum class Entries {
    entries, // ERROR conflicting declarations: the enum entry 'entries' and the property 'Enum.entries' (KT-48872)
    other,
}

fun <T : Comparable<T & Any>> dnnInOwnBound() {} // ERROR left part must be a type parameter with nullable bounds
fun <T : Comparable<T & Any>?> dnnInNullableBound() {}
fun <R : T & Any, T> dnnOfLaterParameter() {}

class Out<out T> {
    context(a: T) // ERROR type parameter T is declared as 'out' but occurs in 'in' position in type T
    fun withContext() {}
}

fun namedTwice(f: (a: Int, a: Int) -> Unit) {} // ERROR duplicate parameter name in a function type
fun annotatedTwice(f: (@ParameterName("a") Int, @ParameterName("a") Int) -> Unit) {} // ERROR duplicate parameter name in a function type
fun underscores(f: (_: Int, _: Int) -> Unit) {} // ERROR duplicate parameter name in a function type
fun distinct(f: (a: Int, b: Int) -> Unit, g: (Int, Int) -> Unit) {}

context(noinline s: String) // ERROR modifier 'noinline' is allowed only for function parameters of an inline function
inline fun noinlineContext(f: () -> Unit) = f()

context(noinline s: String)
fun noinlineContextNotInline() {}

open class Base {
    open val base: Int = 1
}

object StaticOverride : Base() {
    override val base: Int = 2
        @JvmStatic get // ERROR override member cannot be '@JvmStatic' in an object
}

class Lateinits {
    lateinit var x: String

    fun nonLiteral() {
        val p = this::x
        p.isInitialized // ERROR this declaration can only be called on a property literal (e.g. 'Foo::bar')
        (this::x).isInitialized
    }

    inline fun inInline() {
        this::x.isInitialized // ERROR this declaration cannot be used inside an inline function
        run { this::x.isInitialized }
    }

    inline val inlineGetter: Boolean get() = this::x.isInitialized // ERROR this declaration cannot be used inside an inline function
}

open class WithCompanion {
    companion object {
        protected val secret = 1
    }
}

class Sub : WithCompanion() {
    fun read() = WithCompanion.secret // ERROR using non-JVM static members protected in the superclass companion is unsupported yet: 'secret'
}

open class InternalFoo {
    internal open fun foo() {}
}

interface ProtectedFoo {
    protected fun foo() {} // ERROR modifier 'protected' is not applicable inside 'interface'
}

class BothFoos : InternalFoo(), ProtectedFoo {
    override fun foo() {} // ERROR cannot change access privilege 'protected' for 'foo' in 'InternalFoo'
}

@RequiresOptIn
annotation class Marker

fun localSubclassOptIn() {
    @SubclassOptInRequired(Marker::class) // ERROR '@SubclassOptInRequired' is not applicable to a final class
    object {}
}

interface SuspendComponent {
    suspend fun component1(): Int
}

data class NotSuspend(val v: Int) : SuspendComponent // ERROR non-suspend function 'component1' cannot override suspend function 'component1' defined in 'SuspendComponent'

object A {
    object B {
        object A {
            val y = this@A // ERROR ambiguous label: A
        }
    }
}

class Inner {
    inner class Inner {
        fun f() = this@Inner // ERROR ambiguous label: Inner
    }
}

class Nested {
    class Nested {
        fun f() = this@Nested
    }
}

inline fun objectDefault(f: () -> String = object : () -> String { // ERROR invalid default value for an inline functional parameter
    override fun invoke() = ""
}) = f()

open class ProtectedSetter {
    var v: String = ""
        protected set

    inline fun set() {
        v = "x" // ERROR protected property setter call from public-API inline function is prohibited: 'v'
    }

    internal inline fun setInternal() {
        v = "x"
    }
}

enum class CloneableEnum : Cloneable {
    A;

    fun copy(): Any = super.clone() // ERROR many supertypes available
}

annotation class Ann

fun annotationRef() = ::Ann // ERROR annotation class cannot be instantiated

@Target(AnnotationTarget.TYPE)
annotation class TypeAnn

val listed: List<@[Ann] String>? = null // ERROR this annotation is not applicable to target 'type usage'
val listedType: List<@[TypeAnn] String>? = null

@[Deprecated("old") Suppress("UNUSED")]
fun oldStyle() {}

fun fast(f: () -> Unit) = f()
inline fun inlined(f: () -> Unit) = f()

@Retention(AnnotationRetention.SOURCE)
annotation class SourceOnly

fun inlinedLambdaAnnotations() {
    fast @Ann {}
    inlined @Ann {} // ERROR the lambda expression here is an inlined argument, so this annotation cannot be stored anywhere
    inlined @SourceOnly {}
}

fun lambdas(vararg fs: () -> Unit) {}

fun trailingVararg() {
    lambdas({})
    lambdas {} // ERROR passing value as a vararg is only allowed inside a parenthesized argument list
}
