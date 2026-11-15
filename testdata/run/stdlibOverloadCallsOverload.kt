enum class Kind { NONE, WIFI; companion object { fun of(i: Int) = entries[i] } }
class Event(val kind: Kind) {
    companion object {
        @JvmStatic
        fun success(ordinal: Int) =
            success(Kind.of(ordinal))

        @JvmStatic
        fun success(kind: Kind) = Event(kind)
    }
}
object Log {
    fun flow(origin: String, data: Map<String, Any> = emptyMap()) = println("$origin $data")
    fun flow(origin: String, provider: MutableMap<String, Any>.() -> Unit) =
        flow(origin, buildMap { provider() })
}
class Test {
    fun answers(result: String) = answers { result }
    fun answers(result: () -> String) = println(result())
}
fun main() { println(Event.success(1).kind); Log.flow("o") { put("a", 1) }; Test().answers("r") }
