// The receiver of an extension of a type parameter whose type is a
// builder's variable that nothing gives a type, or only another variable
// does, is an error, as in K2.
class Owner<T> {
    fun constrain(t: T) {}
    fun provide(): T = null!!
}

fun <OT> pcla(lambda: (Owner<OT>) -> Unit): OT = null!!
fun <PH> PH.fix() {}
fun <MT> materialize(): MT = null!!
object Target

fun unconstrained() {
    pcla { o ->
        o.provide().fix() // ERROR the type of a receiver hasn't been inferred yet. Specify type argument for generic parameter 'OT' explicitly.
        o.constrain(Target)
    }
}

fun onlyVariableBelow() {
    pcla { o ->
        o.constrain(materialize())
        o.provide().fix() // ERROR the type of a receiver hasn't been inferred yet. Specify type argument for generic parameter 'OT' explicitly.
        o.constrain(Target)
    }
}

fun constrained() {
    pcla { o ->
        o.constrain(Target)
        o.provide().fix()
    }
}
