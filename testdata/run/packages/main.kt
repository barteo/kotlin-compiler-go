@file:Suppress("UNUSED_VARIABLE")

package geo

import geo.shapes.Circle
import geo.shapes.make
import geo.shapes.describe as describeCircle
import geo.util.*

fun main() {
    val c = make(2)
    println(describeCircle(c))
    println(describe(5))
    println(c.area())
    println(shout("hi"))
    println(geo.shapes.describe(geo.shapes.Circle(3)))
    println(geo.shapes.unit.r)
    geo.shapes.created += 10
    println(geo.shapes.created)
    val list: java.util.ArrayList<Circle> = java.util.ArrayList()
    list.add(c)
    println(list.size)
    @Suppress("MagicNumber")
    val n = local() + @Suppress("x") 1
    println(n)
}
