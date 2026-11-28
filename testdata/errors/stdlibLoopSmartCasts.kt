// K2's loops drop the smart casts of the locals they assign with = only,
// not with op=, ++ or --, nor the aliases of those locals at their start.

fun <R> materialize(): R = null!!

fun opAssign() {
    var x: Any? = materialize()
    require(x is String)
    for (i in 1..10) {
        x.length
        x += ""
    }
}

fun assign() {
    var x: Any? = materialize()
    require(x is String)
    for (i in 1..10) {
        x.length // ERROR unresolved reference: length
        x = ""
    }
}

fun alias() {
    val x: Any? = materialize()
    var y = x
    for (i in 1..10) {
        require(y is String)
        x.length
        y += ""
    }
}

// A Boolean val's condition on a local var holds until the var is assigned.
fun condition() {
    var x: Any? = materialize()
    val state = x is String
    for (i in 1..10) {
        if (state) {
            x.length
        }
    }
    x = 1
    if (state) {
        x.length // ERROR unresolved reference
    }
}
