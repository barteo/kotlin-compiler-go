// Enum entries with bodies: each is an instance of a class of its own,
// a subclass of the enum.

interface Shape {
    fun sides(): Int
}

enum class Op(val symbol: String) : Shape {
    PLUS("+") {
        override fun apply(a: Int, b: Int) = a + b
        override fun sides() = 4
    },
    TIMES("*") {
        private val factor = 1
        override fun apply(a: Int, b: Int) = a * b * factor
        override fun sides() = 2
        override fun describe() = "times, " + super.describe()
    },
    MINUS("-") {
        override fun apply(a: Int, b: Int) = a - b
        override fun sides() = 1

        inner class Twice {
            fun of(a: Int, b: Int) = apply(apply(a, b), b)
        }

        override fun describe() = "minus twice: " + Twice().of(10, 3)
    };

    abstract fun apply(a: Int, b: Int): Int

    open fun describe() = "op " + symbol + " #" + ordinal
}

enum class Level(val after: String) {
    LOW(""),
    HIGH("LOW") {
        override val loud = true
        init {
            println("HIGH created after " + after)
        }
    },
    MAX(HIGH.name) {
        override val loud = true
    };

    open val loud = false
}

fun main() {
    for (op in Op.values()) {
        println(op.name + " " + op.apply(6, 3) + " " + op.sides() + " " + op.describe())
    }
    println(Op.valueOf("TIMES").symbol)
    val s: Shape = Op.MINUS
    println(s.sides())
    println(Op.PLUS is Op)
    println(Op.TIMES.toString())
    for (l in Level.values()) {
        println(l.name + " " + l.loud + " " + l.after)
    }
    when (Level.valueOf("MAX")) {
        Level.LOW -> println("low")
        Level.HIGH, Level.MAX -> println("not low")
    }
}
