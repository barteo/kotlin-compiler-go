// As K2, an infix or operator call that a local extension without the
// modifier and a top-level one may both be has an error type: a lambda
// ending with it can't infer its result.
class Example

fun Example.div(other: Example) = 0

fun a(a: Example, b: Example) {
    with(a) { // ERROR cannot infer type for type parameter 'R'
        fun Example.div(other: Example) = ""
        a div b // ERROR 'infix' modifier is required
    }
    run {
        a div b // ERROR 'infix' modifier is required
    }
}
