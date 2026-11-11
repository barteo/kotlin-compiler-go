package lib
interface Metrics {
    fun Map<String, String>.byteCount(): Long
    fun String.basePart(): String
}
class MetricsImpl : Metrics {
    override fun Map<String, String>.byteCount(): Long = entries.sumOf { (it.key.length + it.value.length).toLong() }
    override fun String.basePart(): String = substringBefore("?")
}
