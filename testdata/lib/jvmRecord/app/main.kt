// kotlinc: -jvm-target 17
import records.*

fun main() {
    val r = Rect(2.0, 3.0)
    val s: Shape = r
    println("${r.w} ${r.h} ${r.name} ${r.area} ${s.name} ${s.area} ${r.square}")
    r.scale = 4
    println(r.scale)
    println(r.tag("t"))
    println(r.copy(h = 2.0).square)
    val (w, h) = r
    println(w + h)
    val rec: Record = r
    println(rec == Rect(2.0, 3.0))
    val p = Pair2.twice("x")
    println(p.first + p.second + p)
    println(Pair2(1, "a").second.length)
}
