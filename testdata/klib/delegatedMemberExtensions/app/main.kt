import lib.Metrics
import lib.MetricsImpl

class Listener : Metrics by MetricsImpl() {
    fun size(h: Map<String, String>) = h.byteCount()
    fun base(u: String) = u.basePart()
}

fun main() {
    val l = Listener()
    println(l.size(mapOf("a" to "bc")))
    println(l.base("x?y"))
}
