package io.github.barteo.kc.aa

import com.google.devtools.ksp.impl.KotlinSymbolProcessing
import com.google.devtools.ksp.processing.KSPConfig
import com.google.devtools.ksp.processing.KSPLogger
import com.google.devtools.ksp.processing.KspGradleLogger
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.FileLocation
import com.google.devtools.ksp.symbol.KSNode
import java.io.ByteArrayInputStream
import java.io.ObjectInputStream

/** KSP's run for a build tool, which the loader of jvm/kc-aa/loader calls. */
object KcRun {
    /**
     * Runs KSP with the serialized [config] and returns its exit code, as
     * the KSP fork's KSPLoader does. With [held], the messages that fail
     * the run (the errors, and the warnings where they count as errors)
     * are not printed: their lines, as KSP's Gradle logger writes them,
     * are added to it.
     */
    @JvmStatic
    fun run(config: ByteArray, providers: List<SymbolProcessorProvider>, logLevel: Int, held: MutableList<String>?): Int {
        val kspConfig = ObjectInputStream(ByteArrayInputStream(config)).use { it.readObject() as KSPConfig }
        val printed = KspGradleLogger(logLevel)
        val logger = if (held == null) printed else object : KSPLogger by printed {
            fun hold(severity: Char, message: String, symbol: KSNode?) {
                val at = (symbol?.location as? FileLocation)?.let { "${it.filePath}:${it.lineNumber}: " }.orEmpty()
                held += "$severity: [ksp] $at$message"
            }

            override fun error(message: String, symbol: KSNode?) = hold('e', message, symbol)

            override fun warn(message: String, symbol: KSNode?) =
                if (kspConfig.allWarningsAsErrors) hold('w', message, symbol) else printed.warn(message, symbol)

            override fun exception(e: Throwable) {
                held += "e: [ksp] $e"
            }
        }
        return KotlinSymbolProcessing(kspConfig, providers, logger).execute().ordinal
    }
}
