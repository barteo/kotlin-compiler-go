// A primary constructor parameter that declares a property, var prop, is
// the property in property initializers and init blocks: an implicit
// receiver's member of the same name comes first, and prop = v assigns
// the property. A plain parameter, v, comes before the receiver's member.

class D(val v: Int, var prop: Int)

class C(v: Int, var prop: Int) {
    val applied = D(1, 1).apply { prop = 3 }
    val read = D(5, 6).run { prop }
    val plain = D(5, 6).run { v }
    val lazyApplied by lazy { D(6, 6).apply { prop = 7 } }

    init {
        prop = 4
    }

    fun param(v: Int) = D(8, 9).run { v }
}

fun main() {
    val x = C(0, 1)
    println(x.applied.prop)
    println(x.prop)
    println(x.read)
    println(x.plain)
    println(x.lazyApplied.prop)
    println(x.param(2))
}
