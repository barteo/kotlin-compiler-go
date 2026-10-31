// Enum classes: entries with constructor arguments, members, valueOf,
// name and ordinal, exhaustive when, and enums from the JDK.

enum class Color(val rgb: Int) {
    RED(0xff0000),
    GREEN(0x00ff00),
    BLUE(0x0000ff);

    fun hex(): String = Integer.toHexString(rgb)

    fun next(): Color = when (this) {
        RED -> GREEN
        GREEN -> BLUE
        BLUE -> RED
    }

    companion object {
        fun parse(s: String): Color = valueOf(s.uppercase())
    }
}

enum class Dir { UP, DOWN, LEFT, RIGHT }

interface Opposite {
    fun opposite(): Planet
}

enum class Planet(val mass: Double, val radius: Double) : Opposite {
    MERCURY(3.303e+23, 2.4397e6),
    EARTH(5.976e+24, 6.37814e6);

    val surfaceGravity: Double
        get() = 6.67300E-11 * mass / (radius * radius)

    override fun opposite(): Planet = if (this == EARTH) MERCURY else EARTH
}

fun describe(d: Dir): String = when (d) {
    Dir.UP, Dir.DOWN -> "vertical"
    Dir.LEFT, Dir.RIGHT -> "horizontal"
}

fun unit(u: java.util.concurrent.TimeUnit): String = when (u) {
    java.util.concurrent.TimeUnit.SECONDS -> "s"
    java.util.concurrent.TimeUnit.MILLISECONDS -> "ms"
    else -> "other"
}

fun main() {
    val c = Color.RED
    println(c)
    println(c.name + " " + c.ordinal + " " + c.hex())
    println(c.next().next())
    println(Color.valueOf("BLUE").rgb)
    println(Color.parse("green"))
    println(Color.GREEN == Color.parse("GREEN"))
    println(Color.RED.compareTo(Color.BLUE) < 0)
    println(describe(Dir.LEFT) + " " + describe(Dir.UP))
    println(Planet.EARTH.surfaceGravity > 9.0)
    println(Planet.MERCURY.opposite())
    val o: Opposite = Planet.EARTH
    println(o.opposite().name)
    println(unit(java.util.concurrent.TimeUnit.SECONDS) + unit(java.util.concurrent.TimeUnit.DAYS))
    try {
        Color.valueOf("PINK")
    } catch (e: IllegalArgumentException) {
        println("no PINK")
    }
    val set = java.util.HashSet<Dir>()
    set.add(Dir.UP)
    set.add(Dir.UP)
    println(set.size)
}
