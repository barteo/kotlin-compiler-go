package model

enum class Color { RED, GREEN, BLUE }

class Shape(val name: String) {
    enum class Kind { ROUND, SQUARE }

    companion object {
        const val UNIT = "cm"
        val DEFAULT = Shape("dot")
        fun of(name: String) = Shape(name)
    }
}

object Registry {
    val names = "a,b"
    var count = 0
    fun register(n: Int): Int {
        count += n
        return count
    }
}

enum class Size(val kind: Kind) {
    SMALL(Kind.ROUND), // an entry's arguments see the enum's nested classes
    LARGE(Kind.SQUARE);

    enum class Kind { ROUND, SQUARE }
}
