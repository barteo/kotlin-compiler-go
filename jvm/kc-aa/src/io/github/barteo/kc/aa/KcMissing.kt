package io.github.barteo.kc.aa

import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong

/**
 * The Analysis API members kc doesn't answer, as the generated stubs and
 * the session report them. By default, for KSP, such a member throws
 * UnsupportedOperationException("kc: Class.member"), so that what kc lacks
 * fails with its name. In lenient mode, which [KcStandalone] turns on for a
 * linter (detekt), it returns an empty value of its type instead (null,
 * false, 0, an empty collection) and is counted: a linter's rule then sees
 * nothing rather than failing. -Dkc.strict makes it throw in lenient mode
 * too, and -Dkc.trace prints the counts at exit.
 */
object KcMissing {
    /** Whether unanswered members return empty values: set by [KcStandalone]. */
    @Volatile
    var lenient = false
        internal set

    /** Whether -Dkc.strict is set: an unanswered member throws even in lenient mode. */
    internal val strict = System.getProperty("kc.strict") == "true"

    /** Whether -Dkc.trace is set: prints timings and what kc doesn't answer. */
    internal val trace = System.getProperty("kc.trace") == "true"

    private val counts = ConcurrentHashMap<String, AtomicLong>()

    init {
        if (trace) Runtime.getRuntime().addShutdownHook(Thread { report() })
    }

    internal fun log(msg: String) {
        if (trace) System.err.println("kc: $msg")
    }

    /** [default], for member [what] kc doesn't answer, in lenient mode; otherwise it throws. */
    fun <T> miss(what: String, default: T): T {
        if (!lenient || strict) fail(what)
        counts.computeIfAbsent(what) { AtomicLong() }.incrementAndGet()
        return default
    }

    fun fail(what: String): Nothing = throw UnsupportedOperationException("kc: $what")

    /** What was missed, most often first. */
    fun report() {
        if (counts.isEmpty()) return
        System.err.println("kc: Analysis API members kc doesn't answer (calls):")
        counts.entries.sortedByDescending { it.value.get() }.forEach { (k, v) -> System.err.println("kc:   ${v.get()}\t$k") }
    }
}
