// A member marked @LowPriorityInOverloadResolution gives way to an
// extension that takes the arguments, as SelectBuilder's deprecated
// onTimeout member calls the extension onTimeout.
@file:Suppress("INVISIBLE_REFERENCE", "INVISIBLE_MEMBER")

import kotlin.internal.*

interface Builder {
    fun add(s: String)

    @kotlin.internal.LowPriorityInOverloadResolution
    fun after(millis: Long, block: () -> String): Unit = after(millis, block)

    // Written bare, as kotlinx.coroutines' SelectBuilder.onTimeout is.
    @LowPriorityInOverloadResolution
    fun before(millis: Long, block: () -> String): Unit = before(millis, block)
}

fun Builder.after(millis: Long, block: () -> String) = add("after $millis: ${block()}")
fun Builder.before(millis: Long, block: () -> String) = add("before $millis: ${block()}")

class Log : Builder {
    val lines = mutableListOf<String>()
    override fun add(s: String) {
        lines += s
    }
}

fun main() {
    val log = Log()
    log.after(1) { "explicit" }
    with(log as Builder) { after(2) { "implicit" } }
    (log as Builder).run { this.after(3) { "member" } }
    // The member's own body calls the extension, not itself.
    val f: () -> String = { "reflected" }
    Builder::class.java.getMethod("after", Long::class.java, Function0::class.java).invoke(log, 4L, f)
    Builder::class.java.getMethod("before", Long::class.java, Function0::class.java).invoke(log, 5L, f)
    println(log.lines)
}
