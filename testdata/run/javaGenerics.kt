// Generic Java classes and methods, read from their signatures.
import java.math.BigInteger
import java.util.ArrayList
import java.util.Collections
import java.util.HashMap
import java.util.Optional
import java.util.concurrent.atomic.AtomicReference

class Named<T : Comparable<T>>(val name: String, val key: T)

fun <T : Comparable<T>> larger(a: T, b: T): T = if (a > b) a else b

fun <T : Comparable<T>> keyOf(n: Named<T>): T = n.key

fun sum(xs: ArrayList<Int>): Int {
    var total = 0
    for (i in 0 until xs.size) {
        total += xs.get(i)
    }
    return total
}

fun main() {
    val words = ArrayList<String>()
    words.add("pear")
    words.add("apple")
    words.add("fig")
    println(words.size)
    println(words.get(1).length)
    println(words.get(0).uppercase())
    println(words.contains("fig"))
    println(words.indexOf("apple"))
    println(words.set(2, "plum"))
    println(words)

    // Elements of a list of Int are boxed, and unboxed when used as Int.
    val nums = ArrayList<Int>()
    for (i in 1..5) {
        nums.add(i * i)
    }
    println(sum(nums))
    println(nums.get(4) + 1)

    // The expected type and constructor arguments infer type arguments.
    val copy: ArrayList<String> = ArrayList()
    copy.addAll(words)
    println(copy.size)
    val again = ArrayList(words)
    println(again.get(0).length)

    // Kotlin's Map.get returns V?.
    val ages = HashMap<String, Int>()
    ages.put("ann", 31)
    ages.put("bob", 27)
    println(ages.get("ann"))
    println(ages.get("zed"))
    println(ages.get("bob") ?: 0)
    println((ages.get("zed") ?: 0) + 1)
    println(ages.containsKey("bob"))
    println(ages.size)

    // Iterators.
    val it = words.iterator()
    while (it.hasNext()) {
        print(it.next().length)
    }
    println()

    // Generic static methods.
    println(Collections.max(nums))
    println(Collections.max(nums) + 1)
    println(Collections.min(words))
    println(Collections.emptyList<String>().size)
    println(Collections.singletonList(7).get(0) * 6)
    println(Optional.of("present").get().length)
    println(Optional.ofNullable<String>(null).orElse("absent"))

    val ref = AtomicReference<String>("first")
    println(ref.get().length)
    ref.set("second")
    println(ref.get())

    // Bounds on Java's Comparable.
    println(larger(3, 9))
    println(larger("kiwi", "apple"))
    println(larger(BigInteger.valueOf(5L), BigInteger.TEN))
    println(keyOf(Named("n", 42)) + 1)
    println("b" > "a")
    println(BigInteger.ONE < BigInteger.TEN)
}
