package geo.shapes

class Circle(val r: Int) {
    fun area(): Int = 3 * r * r
}

fun describe(c: Circle): String = "circle r=${c.r}"

val unit = Circle(1)
var created = 0

fun make(r: Int): Circle {
    created++
    return Circle(r)
}
