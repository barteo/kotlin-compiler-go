// get<String>(n) skips the inner receiver's Any.get(name), which takes no
// type arguments, for the outer receiver's get<T>(name).
interface Processor { operator fun <T : CharSequence> get(name: String): T? }
class Matcher { infix fun Any.get(name: String): Int = 7 }
fun <T> with2(p: Processor, block: Processor.() -> T) = p.block()
fun match(block: Matcher.() -> Unit) = Matcher().block()
fun main() {
    val p = object : Processor { @Suppress("UNCHECKED_CAST") override fun <T : CharSequence> get(name: String): T? = name as T }
    with2(p) { match { println(get<String>("n")); println(get("m")) } }
}
