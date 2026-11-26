// A chain of safe calls on a local var smart-casts the properties it
// reads only until the var is assigned, and not where a lambda may
// assign it.
class My(val x: My?)

fun reassigned() {
    var y: My? = My(My(null))
    if (y?.x?.x != null) {
        y.x.x.x
        y = My(null)
        y.x.x.x // ERROR nullable receiver
    }
}

fun reassignedInPlace() {
    var y: My? = My(My(null))
    if (y?.x?.x != null) {
        run { y = My(null) }
        y.x.x.x // ERROR nullable receiver
    }
}

fun captured() {
    var y: My? = My(My(null))
    val f = { y = null }
    if (y?.x?.x != null) {
        y.x.x.x // ERROR nullable receiver
    }
    f()
}
