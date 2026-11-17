// Lambdas as K2's postponed arguments: one with the types of all its
// parameters is analyzed on its own, its type a lower bound of the
// variable it's passed for; a branch of an if or when is a synthetic
// call's argument, joined with the others, and may take their function
// type's parameters, the receiver too.

enum class Color { BLUE }
fun Color.gray() {}
fun useList(list: List<Any?>) {}

fun <S> select(vararg args: S): S = args[0]
fun <I> id(arg: I): I = arg

fun typed() {
    useList(listOf({ x: Color -> x.gray() }, Color.BLUE))
}

fun withNull(b: Boolean) {
    val a = if (b) id { "2" } else null
    val c = select({ "9" }, null)
    val d: (() -> String)? = a ?: c
}

class Foo
typealias X = Foo.(Foo.() -> Unit) -> Unit

fun fromBranches(b: Boolean) {
    val r: String.() -> String = if (b) {{ s: String -> s }} else {{ s -> s + s }}
    val y = when {
        b -> {{ _ -> Unit }}
        else -> null as X
    }
    val z = if (b) {{ x -> x }} else {{ x -> x }} // ERROR cannot infer a type for this parameter
}

open class A<H : Function2<*, *, Function0<String>>>(val f: H)
class B : A<Function2<*, *, Function0<String>>>({ p1: List<*>, p2: String -> { "" } })
class C<T : Function1<*, Any>>(var x: T) {
    val y = C(fun(x: Any): Any = 1) // ERROR expected parameter of type
}

class Box { fun m() = 1 }
operator fun Box.invoke() = 2
class Smart {
    val box: Box? = null
    // kotlin's run(block), not this.run { }: box keeps its smart cast.
    fun f() = if (box != null) run { box.m() + box() } else 0
}
