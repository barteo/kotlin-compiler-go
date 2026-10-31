import java.util.ArrayList
import java.util.Collections
import java.util.Comparator
import java.util.HashMap
import java.util.TreeMap

class Task(val name: String, val priority: Int)

fun main() {
    val xs = ArrayList<Int>()
    xs.add(5)
    xs.add(3)
    xs.add(8)
    xs.add(1)
    Collections.sort(xs) { a, b -> a - b }
    println(xs)
    Collections.sort(xs, Comparator<Int> { a, b -> b - a })
    println(xs)
    xs.removeIf { it > 4 }
    println(xs)
    xs.forEach { println("item $it") }

    var sum = 0
    xs.forEach { sum += it }
    println(sum)
    var big = 10000000000L
    xs.forEach { big += it }
    big += xs.get(0)
    println(big)

    val m = HashMap<String, ArrayList<Int>>()
    m.computeIfAbsent("a") { ArrayList() }.add(1)
    m.computeIfAbsent("a") { ArrayList() }.add(2)
    m.computeIfAbsent("b") { k -> ArrayList() }.add(k(3))
    println(m)
    m.forEach { k, v -> println("$k -> ${v.size}") }

    val t = Thread { println("in thread") }
    t.start()
    t.join()
    val r = Runnable { println("run") }
    r.run()

    val words = ArrayList<String>()
    words.add("pear")
    words.add("fig")
    words.add("banana")
    Collections.sort(words) { a, b -> a.length - b.length }
    println(words)
    words.replaceAll { it.uppercase() }
    println(words)
    println(words.stream().mapToInt { it.length }.sum())
    println(words.stream().filter { it.length > 3 }.count())

    val tasks = ArrayList<Task>()
    tasks.add(Task("write", 2))
    tasks.add(Task("test", 1))
    tasks.add(Task("ship", 3))
    Collections.sort(tasks, byKey { it.priority })
    for (i in 0 until tasks.size) print(tasks.get(i).name + " ")
    println()

    val counts = TreeMap<String, Int>()
    for (i in 0 until words.size) counts.merge(words.get(i).substring(0, 1), 1) { a, b -> a + b }
    counts.merge("P", 1) { a, b -> a + b }
    println(counts)
}

fun k(x: Int): Int = x * 100

fun <T> byKey(key: (T) -> Int): Comparator<T> = Comparator { a, b -> key(a) - key(b) }
