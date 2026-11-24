// As kotlinc, this of a lambda whose receiver's type has a type argument
// that wasn't inferred is reported, without counting against the call it
// is an argument of.
open class Processor<P> { fun process(t: P): Boolean = true }
class Model
fun interface Exec<E> { fun run(model: Model, params: Processor<in E>): Boolean }
fun <C> context(p: Processor<in C>, exec: Exec<C>) {}
fun <C> plain(p: Processor<C>) {}
fun <M> materialize(): Processor<M> = TODO()

fun test(pe: Exec<Model>) {
    materialize().apply { // ERROR cannot infer type for type parameter 'M'
        context(
            this, // ERROR cannot infer type for type parameter 'T'
            pe
        )
    }
    materialize().apply { // ERROR cannot infer type for type parameter 'M'
        plain(this) // ERROR cannot infer type for type parameter
    }
    materialize().apply { // ERROR cannot infer type for type parameter 'M'
        val q = this // ERROR cannot infer type for type parameter 'T'
    }
}
