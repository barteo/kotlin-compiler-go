// An object's @JvmField properties are static fields of the types its
// metadata gives them, not of their erased field descriptors.

import p.Collector
import p.Conf
import p.Keys

class Printing : Collector {
    override fun report(message: String) = "printed $message"
}

fun Conf.collector(): Collector = get(Keys.COLLECTOR) ?: Printing()

fun Conf.name(): String = get(Keys.NAME) ?: "unnamed"

fun main() {
    val conf = Conf()
    println("${conf.collector().report("a")} ${conf.name()}")
    conf.put(Keys.NAME, "conf")
    conf.put(Keys.COLLECTOR, object : Collector {
        override fun report(message: String) = "custom $message"
    })
    println("${conf.collector().report("b")} ${conf.name()} ${Keys.COLLECTOR}")
    val sum: Int = Keys.LIMITS.sum()
    Keys.LIMITS = Keys.LIMITS + 3
    println("${Keys.PREFIX}$sum ${Keys.LIMITS.last()}")
}
