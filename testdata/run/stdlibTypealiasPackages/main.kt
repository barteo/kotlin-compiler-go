package geo

import geo.defs.Figure
import geo.defs.Point
import geo.defs.Points
import geo.defs.Transform

fun shift(p: Point, t: Transform): Point = t(p)

fun main() {
    val ps = Points()
    ps.add(1 to 2)
    println(shift(ps[0]) { (x, y) -> x + 1 to y * 2 })
    val f = Figure("square")
    println(f.name)
    val g: geo.defs.Figure = geo.defs.Figure("circle")
    println(g.name)
}
