import mf.LIMIT
import mf.bump
import mf.counter
import mf.digits
import mf.doubled
import mf.lazyName
import mf.processors
import mf.settable
import mf.withPublished
import mf.half
import mf.pad
import mf.scaledSum
import mf.shout
import mf.twice

fun main() {
    println(shout("hi"))
    println(pad("ab") + "|" + pad("ab", 3) + "|" + pad("ab", 4, '-'))
    var n = 0
    println(twice { ++n })
    println(half(8))
    try {
        half(3)
    } catch (e: IllegalStateException) {
        println(e.message)
    }
    println(12345.digits())
    println(scaledSum(listOf(1, 2, 3)))
    println(processors)
    println(bump())
    counter += 10
    println(counter)
    println(doubled)
    settable = "b"
    println(settable)
    println(lazyName)
    println(LIMIT)
    println(withPublished())
    println(::counter.get())
    // The facade's methods, as Java calls them.
    val facade = Class.forName("mf.Text")
    println(facade.getMethod("shout", String::class.java).invoke(null, "x"))
    println(facade.methods.map { it.name }.filter { !it.contains("$") && it != "getClass" && it in setOf("shout", "pad", "half", "digits", "scale", "scaledSum", "twice", "getProcessors", "getCounter", "setCounter", "getDoubled", "getLazyName", "getSettable", "setSettable") }.sorted())
}
