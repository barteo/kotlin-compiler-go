// this of a lambda whose receiver's type is a type variable of the call
// that can't be inferred is reported as the variable is, where it is
// written: it has no type to be used with.
class Target

fun consume(value: Target) {}

fun <PTV> build(block: PTV.() -> Unit): PTV = null!!

fun argument() {
    val t = build { // ERROR cannot infer type for type parameter 'PTV'
        consume(this) // ERROR cannot infer type for type parameter 'PTV'
    }
}

fun labeled() {
    val t = build { // ERROR cannot infer type for type parameter 'PTV'
        val x = this@build // ERROR cannot infer type for type parameter 'PTV'
    }
}

// Not where the variable is inferred.
fun inferred(): Target = build {
    consume(this)
}

fun explicit() {
    build<Target> { consume(this) }
}
