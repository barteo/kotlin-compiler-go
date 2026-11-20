package p

import kotlin.reflect.KClass
import p.Outer.Marker
import p.Outer.Marker.Nested // ERROR can only be used as an annotation

@RequiresOptIn
@Retention(AnnotationRetention.BINARY)
annotation class Exp

object Outer {
    @RequiresOptIn
    annotation class Marker {
        class Nested
        companion object {
            const val value = 1
        }
    }
}

enum class E {
    Plain,
    @Exp Marked,
}

annotation class Uses(val e: E)
annotation class Classes(vararg val k: KClass<*>)

val plain = E.Plain
val marked = E.Marked // ERROR needs opt-in

@Uses(E.Marked) // ERROR needs opt-in
fun usesMarked() {}

@OptIn(Exp::class)
@Uses(E.Marked)
fun optedIn() {}

@Classes(RequiresOptIn::class) // ERROR can only be used as an annotation
fun classLit() {}

val viaQualifier = Outer.Marker.value // ERROR can only be used as an annotation

@Exp
open class C {
    fun member() {}
}

@OptIn(Exp::class)
fun withC(f: C.() -> Unit) {}

fun implicitThis() {
    withC { // ERROR needs opt-in
        member() // ERROR needs opt-in
    }
}

data class D(@property:Exp val x: Int)

fun destructure(d: D) {
    val (x) = d // ERROR needs opt-in
}

fun interface F {
    @Exp
    fun run()
}

fun sam() {
    val f = F {} // ERROR needs opt-in
}

@Exp
fun samOptedIn() {
    val f = F {}
}
