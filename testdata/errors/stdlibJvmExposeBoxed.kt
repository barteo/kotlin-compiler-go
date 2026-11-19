@file:OptIn(ExperimentalStdlibApi::class)

@JvmInline
value class IC(val s: String)

@JvmExposeBoxed("foo") // ERROR with name is applicable only to functions
class Foo

@JvmExposeBoxed // ERROR requires a name
fun boxedRet(): IC = TODO()

@JvmExposeBoxed("boxedRet2")
fun boxedRetNamed(): IC = TODO()

@JvmExposeBoxed
fun takes(ic: IC) {}

@JvmExposeBoxed
fun result(r: Result<Any>) {}

@JvmExposeBoxed("a/b") // ERROR illegal JVM name
fun slash(ic: IC) {}

@JvmExposeBoxed("same" + "Name") // ERROR cannot coincide with that of the declaration
fun sameName(): IC = TODO()

@JvmExposeBoxed("foo") // ERROR cannot coincide with the one in '@JvmName'
@JvmName("foo")
fun renamed(): IC = TODO()

@JvmExposeBoxed // ERROR suspend functions cannot be exposed
suspend fun susp(ic: IC) {}

@JvmExposeBoxed // ERROR reified
inline fun <reified T> reified(ic: IC) {}

@JvmSynthetic
@JvmExposeBoxed // ERROR synthetic
fun synth(ic: IC) {}

val getterTop: IC
    @JvmExposeBoxed get() = TODO() // ERROR requires a name

@JvmExposeBoxed // ERROR open or abstract
interface I {
    @JvmExposeBoxed // ERROR open or abstract
    fun f(ic: IC)
}

abstract class A {
    @JvmExposeBoxed // ERROR open or abstract
    open fun g(ic: IC) {}

    @JvmExposeBoxed
    fun h(ic: IC) {}
}

class C : I {
    @JvmExposeBoxed
    override fun f(ic: IC) {}

    @JvmExposeBoxed
    fun boxedRet(): IC = TODO()

    @get:JvmExposeBoxed
    val g: IC get() = TODO()
}

fun withLocal() {
    @JvmExposeBoxed // ERROR local functions
    fun local(ic: IC) {}
}
