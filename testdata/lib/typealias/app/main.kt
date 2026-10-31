import defs.Box
import defs.Handler
import defs.IntBox
import defs.Pairs
import defs.Pred
import defs.Registry
import defs.StrMap
import defs.apply
import defs.build
import defs.filter
import defs.registry
import defs.twice

fun main() {
    val h: Handler = { it.length }
    println(apply(h, "three"))
    val even: Pred<Int> = { it % 2 == 0 }
    println(filter(listOf(1, 2, 3, 4), even))
    println(build(3) { n -> repeat(n) { append('*') } })
    val r: Registry = registry()
    r["len"] = h
    println(r.getValue("len")("ab"))
    val m: StrMap<Int> = mapOf("a" to 1)
    println(m["a"])
    val ps: Pairs<Int> = listOf(1 to 2)
    println(ps)
    val b: IntBox = Box(4)
    println(b.map { it * 2 }.v)
    val inc: Box.Mapper = { it + 1 }
    println(twice(inc, 1))
}
