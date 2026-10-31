import java.util.Collections
import java.util.LinkedList

// Kotlin's collection types, which are the java.util interfaces on the JVM.

fun total(xs: List<Int>): Int {
    var sum = 0
    for (x in xs) {
        sum += x
    }
    return sum
}

fun firstOrDefault(xs: List<String>, d: String): String {
    if (xs.isEmpty()) {
        return d
    }
    return xs[0]
}

fun fill(xs: MutableList<Int>, n: Int) {
    for (i in 1..n) {
        xs.add(i * i)
    }
}

fun main() {
    val nums = java.util.ArrayList<Int>()
    fill(nums, 4)
    println(nums)
    println(total(nums))
    println(nums.size)
    nums[0] = 100
    println(nums[0] + nums[1])
    println(nums.removeAt(1))
    println(nums.indexOf(16))
    println(nums.contains(100))
    for (x in nums) {
        print(x)
        print(" ")
    }
    println()

    // A Java method's List is a MutableList that also takes read-only lists.
    val ro: List<Int> = nums
    val view = Collections.unmodifiableList(ro)
    println(view.size)
    println(firstOrDefault(Collections.emptyList(), "none"))
    println(firstOrDefault(LinkedList<String>(), "empty"))

    val ages = HashMap<String, Int>()
    ages.put("ann", 31)
    ages.put("bob", 27)
    println(ages["ann"])
    println(ages["nobody"])
    println(ages.size)
    var sum = 0
    for (e in ages.entries) {
        sum += e.value
    }
    println(sum)
    for (k in ages.keys) {
        print(k.length)
    }
    println()
    println(ages.values.size)
    println(ages.containsKey("bob"))

    val m: Map<String, Int> = ages
    val got = m.get("bob")
    println(got)

    // JDK members of the collection types.
    val jl = java.util.List.of(4, 5, 6)
    println(jl[2])
    jl.forEach { print(it) }
    println()
    println(jl.stream().count())

    val it = nums.iterator()
    while (it.hasNext()) {
        if (it.next() == 9) {
            it.remove()
        }
    }
    println(nums)
}
