// RunKsp runs KSP's processing loop, on a chosen backend, with the symbol
// processors on its class path (found with ServiceLoader), and prints the
// files they generate, relative to OUT_DIR.
//
//	RunKsp SRC_DIR[:SRC_DIR...] OUT_DIR [LIB[:LIB...]] (-Dksp.backend=kc|aa)
//
// LIBs are the module's libraries; system properties ksp.arg.NAME=VALUE
// are processor options, and ksp.languageVersion (default 2.4) is the
// language and API version: stock KSP 2.3.x's needs 2.3.
import com.google.devtools.ksp.impl.KotlinSymbolProcessing
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.google.devtools.ksp.processing.KSPLogger
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.KSNode
import java.io.File
import java.util.ServiceLoader
import kotlin.system.exitProcess

object RunLogger : KSPLogger {
    var errors = 0
    override fun logging(message: String, symbol: KSNode?) {}
    override fun info(message: String, symbol: KSNode?) {}
    override fun warn(message: String, symbol: KSNode?) = System.err.println("w: $message")
    override fun error(message: String, symbol: KSNode?) {
        errors++
        System.err.println("e: $message")
    }
    override fun exception(e: Throwable) {
        errors++
        e.printStackTrace()
    }
}

fun main(args: Array<String>) {
    if (args.size < 2) {
        System.err.println("usage: RunKsp SRC_DIR[:SRC_DIR...] OUT_DIR [LIB[:LIB...]]")
        exitProcess(2)
    }
    val srcs = args[0].split(File.pathSeparator).filter { it.isNotEmpty() }.map { File(it).absoluteFile }
    val out = File(args[1]).absoluteFile
    val libs = args.getOrNull(2)?.split(File.pathSeparator)?.filter { it.isNotEmpty() }?.map(::File) ?: emptyList()
    val options = System.getProperties().stringPropertyNames().filter { it.startsWith("ksp.arg.") }
        .associate { it.removePrefix("ksp.arg.") to System.getProperty(it) }
    val config = KSPJvmConfig.Builder().apply {
        moduleName = "main"
        sourceRoots = srcs
        javaSourceRoots = emptyList()
        commonSourceRoots = emptyList()
        libraries = libs
        jdkHome = File(System.getProperty("java.home"))
        projectBaseDir = srcs.first()
        outputBaseDir = out
        cachesDir = File(out, "caches")
        classOutputDir = File(out, "classes")
        kotlinOutputDir = File(out, "kotlin")
        javaOutputDir = File(out, "java")
        resourceOutputDir = File(out, "resources")
        languageVersion = System.getProperty("ksp.languageVersion", "2.4")
        apiVersion = languageVersion
        jvmTarget = "17"
        jvmDefaultMode = "enable"
        incremental = false
        processorOptions = options
    }.build()
    val providers = ServiceLoader.load(SymbolProcessorProvider::class.java).toList()
    if (providers.isEmpty()) {
        System.err.println("RunKsp: no SymbolProcessorProvider on the class path")
        exitProcess(2)
    }
    val code = KotlinSymbolProcessing(config, providers, RunLogger).execute()
    for (dir in listOf("kotlin", "java", "resources")) {
        File(out, dir).walkTopDown().filter { it.isFile }.sorted().forEach { println(it.relativeTo(out).path) }
    }
    if (code != KotlinSymbolProcessing.ExitCode.OK || RunLogger.errors > 0) {
        System.err.println("KSP exited with $code")
        exitProcess(1)
    }
    // Stock KSP leaves non-daemon threads behind.
    exitProcess(0)
}
