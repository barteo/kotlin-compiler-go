package util

// A Java map whose get is annotated @NonNull returns a non-null value.
object Defaults {
    fun describe(): String {
        val m = DefaultMap<String, String>("none")
        m["a"] = "x"
        val a: String = m["a"]
        val b: String = m.get("b")
        return "$a ${b.length}"
    }
}
