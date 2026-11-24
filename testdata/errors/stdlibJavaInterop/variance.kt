// JAVA_TYPE_MISMATCH: a Java collection parameter takes a read-only
// collection of a subtype, but a mutable one only of its element type.

fun variance(v: Variance, ml: MutableList<String>, l: List<String>, ma: MutableList<Any>) {
    v.list(ml) // ERROR Java type mismatch
    v.list(l)
    v.list(ma)
    v.list(ml as MutableList<Any>)
    v.any(ml)
    v.generic(ml)
    v.raw(ml)
}

fun nested(v: Variance, mll: MutableList<MutableList<String>>, mlr: MutableList<List<Any>>, mla: MutableList<ArrayList<Any>>) {
    v.lists(mll) // ERROR Java type mismatch
    v.lists(mlr)
    v.lists(mla) // ERROR Java type mismatch
}

fun projected(v: Variance, out: MutableList<out Number>, star: MutableList<*>, stars: MutableList<MutableList<*>>) {
    v.list(out) // ERROR Java type mismatch
    v.list(star)
    v.list(stars) // ERROR Java type mismatch
}
