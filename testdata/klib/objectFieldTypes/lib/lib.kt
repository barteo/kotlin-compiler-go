package p

interface Collector {
    fun report(message: String): String
}

object Keys {
    @JvmField val COLLECTOR = Key.create<Collector>("collector")
    @JvmField val NAME: Key<String> = Key("name")
    @JvmField var LIMITS: List<Int> = listOf(1, 2)
    const val PREFIX = "k:"
}
