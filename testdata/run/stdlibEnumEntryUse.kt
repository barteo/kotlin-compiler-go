// A finally block (use's) and a try (runCatching's) in enum entry arguments.
import java.io.StringReader
enum class Read(val n: Int) {
    X(StringReader("x").use { it.read() }), Y(runCatching { error("e") }.getOrDefault(5))
}
fun main() { for (r in Read.values()) println("$r ${r.n}") }
