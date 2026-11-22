// A type parameter that an argument gives a type made of one that
// couldn't be inferred can't be inferred either, as in kotlinc, whether
// or not its type argument is written.
class TargetType
class Buildee<TV>
class Box<X>(val x: X)

fun <PTV> build(instructions: Buildee<PTV>.() -> Unit): Buildee<PTV> = Buildee<PTV>()
fun <T> id(x: T) {}
fun <T> two(x: T, y: T) {}
fun <T> boxed(x: Box<T>) {}
fun <T> bt(x: Buildee<T>) {}
fun plain(x: Buildee<TargetType>) {}
fun any(x: Any) {}

fun test() {
    val buildee = build {} // ERROR cannot infer type for type parameter 'PTV'
    id<Buildee<TargetType>>(buildee) // ERROR cannot infer type for type parameter 'T'
    id<Any>(buildee) // ERROR cannot infer type for type parameter 'T'
    id(buildee) // ERROR cannot infer type for type parameter 'T'
    two<Buildee<TargetType>>(buildee, buildee) // ERROR cannot infer type for type parameter 'T'
    boxed<Buildee<TargetType>>(Box(buildee)) // ERROR cannot infer type for type parameter 'T'
    bt(buildee) // ERROR cannot infer type for type parameter 'T'

    // The type with the error is the type parameter's own: it gives it
    // nothing. And a parameter that isn't generic takes it.
    bt<TargetType>(buildee)
    plain(buildee)
    any(buildee)
    val x: Buildee<TargetType> = buildee
    id<Buildee<TargetType>>(build {})
    id<Buildee<TargetType>>(Buildee<TargetType>())
}

fun <T> T.ext() {}
fun <T> T.run2(f: T.() -> Unit) {}
fun <T> T.let2(f: (T) -> Unit) {}
fun <T> with2(x: T, f: T.() -> Unit) {}

// An extension's receiver gives its type as an argument does, and this
// of a lambda whose receiver is of such a type parameter is reported.
fun receivers() {
    val p = build {} // ERROR cannot infer type for type parameter 'PTV'
    p.ext() // ERROR cannot infer type for type parameter 'T'
    p.run2 { // ERROR cannot infer type for type parameter 'T'
        any(this) // ERROR cannot infer type for type parameter 'T'
    }
    p.let2 { // ERROR cannot infer type for type parameter 'T'
        any(it)
    }
    with2(p) { // ERROR cannot infer type for type parameter 'T'
        this.hashCode() // ERROR cannot infer type for type parameter 'T'
        hashCode()
    }
    p.hashCode()
}
