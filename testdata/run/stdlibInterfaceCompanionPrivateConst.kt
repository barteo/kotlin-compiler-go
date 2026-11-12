// An interface's fields are public: its companion's private consts stay
// in the companion's class, and its public ones are in both.
interface Dao {
    companion object {
        private const val TAG = "Dao"
        const val PUB = "pub"
        internal const val INT = 3
        private val list = listOf(1)

        fun tag() = TAG + list
    }

    fun name(): String = TAG + PUB + INT
}

fun main() {
    println(object : Dao {}.name() + Dao.tag())
    println(Dao::class.java.declaredFields.map { it.name }.sorted())
}
