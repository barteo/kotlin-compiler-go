import java.util.Properties

class Box(val name: String) {
    @Suppress("USELESS_ELVIS")
    fun orDefault(): String = name ?: "default"
}

fun lookup(p: Properties?, q: Properties): String? =
    p?.getProperty("a", null) ?: q.getProperty("b", null) ?: q.getProperty("c", null)

// A value that isn't null to Kotlin may be all the same, set so by
// reflection or by Gson: ?: and == null check it, as kotlinc's do.
@Suppress("SENSELESS_COMPARISON")
fun strip(xs: List<String>): List<String> = xs.filter { it != null }

@Suppress("SENSELESS_COMPARISON")
fun nulls(xs: List<String>): Int = xs.count { it == null }

fun main() {
    val b = Box("x")
    Box::class.java.getDeclaredField("name").apply { isAccessible = true; set(b, null) }
    println(b.orDefault())
    println(lookup(null, Properties()))
    println(lookup(Properties().apply { setProperty("a", "1") }, Properties()))
    @Suppress("UNCHECKED_CAST")
    val xs = listOf("a", null, "b") as List<String>
    println(strip(xs))
    println(nulls(xs))
}
