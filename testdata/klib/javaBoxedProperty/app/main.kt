import p.Counter
import p.describe

fun make(count: Int? = null, scale: Double? = null, limit: Int? = null): Counter =
    Counter().apply {
        this.count = count
        this.scale = scale
        this.limit = limit
    }

fun main() {
    println(describe(make()))
    val c = make(3, 1.5, 4)
    c.count += 2
    c.count++
    ++c.count
    c.scale = c.scale * 2
    c.limit += 1
    c.limit++
    println(describe(c))
    println("${c.count--} ${c.count} ${--c.limit}")
    c.apply {
        count = null
        scale = null
        limit = null
    }
    println(describe(c))
    c.label = Counter.parse(null)
    println(c.label == null)
    c.label += "x"
    c.apply { label += "y" }
    c.label = Counter.parse(" z ")
    c.label += "!"
    println(c.label)
}
