// A lambda's parameter declared without a type that nothing gives one
// has, as in kotlinc, Any's members and the extensions of any receiver:
// another member is unresolved.
class C { fun member() {}; val prop = 1 }
class Container<A> { fun consume(arg: A) {} }

fun any(x: Any) {}
fun <B> build(func: (Container<B>) -> B) {}
val C.ep get() = 1
fun C.plain() {}
fun <T> T.gen() {}

fun test(b: Boolean) {
    any { x -> x.prop } // ERROR unresolved reference: prop
    any { x -> x.member() } // ERROR unresolved reference: member
    any { x -> x.nope() } // ERROR unresolved reference: nope
    any { x -> x.prop.inc() } // ERROR unresolved reference: prop
    any { x, y -> y.prop } // ERROR unresolved reference: prop
    val f = { x -> x.prop } // ERROR unresolved reference: prop
    any { x -> x.ep } // ERROR cannot infer a type for this parameter
    any { x -> x.plain() } // ERROR cannot infer a type for this parameter
    any { x -> x.toString() } // ERROR cannot infer a type for this parameter
    any { x -> x.gen() } // ERROR cannot infer type for type parameter 'T'
    build { container -> // ERROR cannot infer type for type parameter 'B'
        if (b) {
            return@build { arg -> // ERROR cannot infer a type for this parameter
                arg.prop // ERROR unresolved reference: prop
            }
        }
        container
    }
}
