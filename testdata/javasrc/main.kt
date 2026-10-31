import app.Named
import app.Square
import util.Registry

// The module's Java classes, which kc reads from their sources.
const val SIDES = Square.SIDES + 1

@Suppress(Square.KIND)
fun main() {
    val s = Square(2.0)
    println(s)
    println(s.label())
    println("${Square.SIDES} ${Square.CORNERS} ${Square.KIND} ${Square.BIG} ${Square.FIRST} ${Square.LAST} ${Square.HALF}")
    println(SIDES)
    when (4) {
        Square.SIDES -> println("four sides")
        else -> println("other")
    }
    if (Square.DEBUG) println("debug")
    println(Square.twice("a"))
    println(Square.sum(1, 2, 3))
    println(Square.describe(s))
    val b = Square.Builder().side(3.0).build()
    println(b.area())
    for (c in Square.Color.values()) println("${c.name} ${c.code()}")
    println(Square.Color.valueOf("GREEN").ordinal)
    println(s.accept(Square.Visitor { it.side * 10 }))
    val v = object : Square.Visitor<String> {
        override fun visit(s: Square) = "visited ${s.side}"
    }
    println(s.accept(v) + " by " + v.name())
    s.twin = b
    println(s.twin)
    println(s.side)
    val r = Registry<Square>()
    r.add(s)
    r.add(b)
    println(Registry.PREFIX)
    println(r.all().map { it.area() })
    println(r.first().label())
    val named: Named = s
    println(named.label())
    widgets()
}
