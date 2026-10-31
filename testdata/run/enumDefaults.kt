// Default values of an enum's constructor parameters: entries that omit
// arguments call the synthetic constructor that takes a mask, after the
// entry's name and ordinal, as kotlinc's do; so do entries with bodies.

enum class Size(val w: Int = 1, val h: Int = w * 2, val label: String = "s$w") {
    SMALL,
    WIDE(4),
    TALL(h = 9),
    NAMED(label = "n", w = 3),
    ODD(5) {
        override fun area() = -w * h
    },
    LAZY {
        val f: () -> String = { label + ordinal }
        override fun toString() = f()
    };

    open fun area() = w * h
}

enum class Mode(val run: (Int) -> Int = { it + 1 }) {
    INC,
    DOUBLE({ it * 2 }),
}

fun main() {
    for (s in Size.values()) {
        println("$s ${s.w} ${s.h} ${s.label} ${s.area()}")
    }
    println(Mode.INC.run(3) + Mode.DOUBLE.run(3))
}
