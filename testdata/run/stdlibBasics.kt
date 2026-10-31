import java.util.Collections
import kotlin.random.Random

// Top-level functions and classes of the Kotlin standard library, read
// from its metadata.

fun describe(p: Pair<String, Int>): String = p.first + " is " + p.second

fun main() {
    // println resolves to kotlin.io's overloads, whose bodies kc emits.
    println("text")
    println(42)
    println(7000000000L)
    println(2.5)
    println(false)
    println(null)
    println()
    print("no newline")
    println()
    val n: Int? = 3
    println(n)

    // Classes: constructors and properties from metadata.
    val p = Pair("answer", 42)
    println(p.first)
    println(p.second + 1)
    println(p)
    println(describe(Pair("x", 1)))
    val t = Triple(1, 2L, "three")
    println(t.third.length)
    val r = Regex("a+b")
    println(r.matches("aaab"))
    println(r.matches("ba"))

    // Functions.
    println(maxOf(3, 7))
    println(minOf(4L, 2L))
    val one = listOf("only")
    println(one)
    println(Collections.unmodifiableList(one).size)
    val rnd = Random(42)
    val a = rnd.nextInt(100)
    println(a >= 0 && a < 100)

    // Typealiases: kotlin.collections.ArrayList is java.util.ArrayList.
    val names = ArrayList<String>()
    names.add("b")
    names.add("a")
    Collections.sort(names)
    println(names)
    val counts = HashMap<String, Int>()
    counts.put("x", 1)
    println(counts)

    // Lambdas are kotlin.jvm.functions.FunctionN.
    val f: (Int) -> Int = { it * 2 }
    println(f(21))
    val g: () -> Unit = { println("unit lambda") }
    g()
}
