package io.github.barteo.kc.detekt

import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong

/**
 * The Analysis API members kc doesn't answer, as the generated stubs and
 * the session report them: each returns an empty value of its type and is
 * counted, or throws under -Dkc.strict. The counts are printed at exit
 * with -Dkc.trace.
 */
object KcMissing {
    private val counts = ConcurrentHashMap<String, AtomicLong>()

    init {
        if (KcDetekt.trace) {
            Runtime.getRuntime().addShutdownHook(Thread { report() })
        }
    }

    fun <T> miss(what: String, default: T): T {
        if (KcDetekt.strict) fail(what)
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
 * Under -Dkc.trace, how often the session answers each kind of question
 * and for how many distinct subjects: the repeats are what a cache saves.
 */
internal object KcStats {
    private val calls = ConcurrentHashMap<String, AtomicLong>()
    private val subjects = ConcurrentHashMap<String, MutableSet<Any>>()

    init {
        if (KcDetekt.trace) Runtime.getRuntime().addShutdownHook(Thread { report() })
    }

    fun count(what: String, subject: Any?) {
        if (!KcDetekt.trace) return
        calls.computeIfAbsent(what) { AtomicLong() }.incrementAndGet()
        if (subject != null) {
            subjects.computeIfAbsent(what) { java.util.Collections.newSetFromMap(java.util.IdentityHashMap<Any, Boolean>()).let { java.util.Collections.synchronizedSet(it) } }.add(subject)
        }
    }

    private fun report() {
        if (calls.isEmpty()) return
        System.err.println("kc: questions the session answered (calls, distinct subjects, repeats):")
        calls.entries.sortedByDescending { it.value.get() }.forEach { (k, v) ->
            val n = v.get(); val d = subjects[k]?.size ?: 0
            System.err.println("kc:   $n\t$d\t${if (n > 0) 100 * (n - d) / n else 0}%\t$k")
        }
    }
}
