package b

// Entries with an initializer: arguments, a body, or an annotation.
enum class Level(val rank: Int) {
    LOW(1),
    HIGH(2) {
        override fun label(): String = "high"
        override val short: String get() = "H"
        val extra: Int = 1
        fun own(a: String): Int = rank
    };

    open fun label(): String = "low"
    open val short: String get() = "L"
}

enum class Step { ONE, TWO(), THREE {}, @Deprecated("") FOUR }
