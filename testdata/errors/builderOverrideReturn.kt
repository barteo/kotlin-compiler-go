// An override in a builder's lambda is checked with the builder's
// variable unfixed, as in K2: it doesn't infer the variable.
class Target

interface Source {
    fun produce(): Buildee<Target>
}

class Buildee<TV>

fun <PTV> build(instructions: Buildee<PTV>.() -> Unit): Buildee<PTV> {
    val b = Buildee<PTV>()
    b.instructions()
    return b
}

fun main() {
    build { // ERROR cannot infer type for type parameter 'PTV'
        object : Source {
            override fun produce() = this@build // ERROR return type of 'produce' is not a subtype
        }
    }
}
