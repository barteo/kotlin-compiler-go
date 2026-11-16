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
