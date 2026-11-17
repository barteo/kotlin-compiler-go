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

/**
 * Under -Dkc.trace, how often a linter's session answers each kind of
 * question and for how many distinct subjects: the repeats are what a
 * cache saves.
 */
internal object KcStats {
    private val calls = ConcurrentHashMap<String, AtomicLong>()
    private val subjects = ConcurrentHashMap<String, MutableSet<Any>>()

    init {
        if (KcMissing.trace) Runtime.getRuntime().addShutdownHook(Thread { report() })
    }

    fun count(what: String, subject: Any?) {
        if (!KcMissing.trace) return
        calls.computeIfAbsent(what) { AtomicLong() }.incrementAndGet()
        if (subject != null) {
            subjects.computeIfAbsent(what) { java.util.Collections.synchronizedSet(java.util.Collections.newSetFromMap(java.util.IdentityHashMap())) }.add(subject)
        }
    }

    private fun report() {
        if (calls.isEmpty()) return
        System.err.println("kc: questions the session answered (calls, distinct subjects, repeats):")
        calls.entries.sortedByDescending { it.value.get() }.forEach { (k, v) ->
            val n = v.get()
            val d = subjects[k]?.size ?: 0
            System.err.println("kc:   $n\t$d\t${if (n > 0) 100 * (n - d) / n else 0}%\t$k")
        }
    }
}
