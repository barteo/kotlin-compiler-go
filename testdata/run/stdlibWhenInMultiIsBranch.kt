sealed interface Mode
data class Free(val n: Int) : Mode
data class Operation(val key: String) : Mode
data class OffRoad(val key: String) : Mode
object Idle : Mode

fun keyOf(m: Mode): String {
    when (m) {
        is Free -> return "free ${m.n}"
        is Operation, is OffRoad -> {
            val key: String = when (m) {
                is Operation -> m.key
                is OffRoad -> "off " + m.key
            }
            return key
        }
        else -> return "other"
    }
}

fun main() {
    for (m in listOf(Free(1), Operation("a"), OffRoad("b"), Idle)) println(keyOf(m))
}
