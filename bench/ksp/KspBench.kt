// KspBench times KSP's processing loop, run after run in one JVM, as a
// Gradle daemon would run kspKotlin: session setup (until the processor
// provider's create is called), the processing rounds, finish and
// teardown. It prints a line per run and then the cold (first) run, the
// warm runs' median (of the last half) and the fastest run.
//
//	KspBench SRC_DIR[:SRC_DIR...] LIBS_FILE RUNS
//
// LIBS_FILE lists the module's libraries, one per line. The symbol
// processors are found on the class path with ServiceLoader. System
// properties: ksp.backend (the KSP fork's backend, kc or aa),
// ksp.languageVersion (default 2.4; stock KSP 2.3.x's needs 2.3), and
// jfr.from=N, which records execution samples from run N on into
// bench.jfr (see Classify.java).
//
// It is built against the KSP fork (jvm/kc-aa/build.sh) and against stock
// KSP; bench/ksp/run.sh runs both.
import com.google.devtools.ksp.impl.KotlinSymbolProcessing
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.google.devtools.ksp.processing.KSPLogger
import com.google.devtools.ksp.processing.Resolver
import com.google.devtools.ksp.processing.SymbolProcessor
import com.google.devtools.ksp.processing.SymbolProcessorEnvironment
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.KSAnnotated
import com.google.devtools.ksp.symbol.KSNode
import java.io.File
import java.nio.file.Files
import java.time.Duration
import java.util.ServiceLoader
import kotlin.system.exitProcess

/** The phase the main thread is in; Classify reads it from the stack instead. */
@Volatile
var phase = "setup"

class Times {
    var start = 0L
    var create = 0L
    var finishStart = 0L
    var finishEnd = 0L
    val rounds = mutableListOf<Pair<Long, Long>>()
}

private var times = Times()

/** Wraps a provider to time its processor's rounds. */
class TimedProvider(private val p: SymbolProcessorProvider) : SymbolProcessorProvider {
    override fun create(environment: SymbolProcessorEnvironment): SymbolProcessor {
        val t = times
        t.create = System.nanoTime()
        phase = "create"
        val sp = p.create(environment)
        phase = "between"
        return TimedProcessor(sp, t)
    }
}

class TimedProcessor(private val sp: SymbolProcessor, private val times: Times) : SymbolProcessor {
    override fun process(resolver: Resolver): List<KSAnnotated> {
        val s = System.nanoTime()
        phase = "process"
        try {
            return sp.process(resolver)
        } finally {
            times.rounds += s to System.nanoTime()
            phase = "between"
        }
    }

    override fun finish() {
        times.finishStart = System.nanoTime()
        phase = "finish"
        sp.finish()
        times.finishEnd = System.nanoTime()
        phase = "teardown"
    }

    override fun onError() = sp.onError()
}

object BenchLogger : KSPLogger {
    var errors = 0
    override fun logging(message: String, symbol: KSNode?) {}
    override fun info(message: String, symbol: KSNode?) {}
    override fun warn(message: String, symbol: KSNode?) {}
    override fun error(message: String, symbol: KSNode?) {
        errors++
        System.err.println("e: $message")
    }
    override fun exception(e: Throwable) {
        errors++
        e.printStackTrace()
    }
}

private fun ms(ns: Long) = ns / 1_000_000

fun main(args: Array<String>) {
    if (args.size != 3) {
        System.err.println("usage: KspBench SRC_DIR[:SRC_DIR...] LIBS_FILE RUNS")
        exitProcess(2)
    }
    val srcs = args[0].split(File.pathSeparator).filter { it.isNotEmpty() }.map { File(it).absoluteFile }
    val libs = File(args[1]).readLines().filter { it.isNotBlank() }.map(::File)
    val runs = args[2].toInt()
    val backend = System.getProperty("ksp.backend") ?: "stock"
    val jfrFrom = Integer.getInteger("jfr.from", 0)
    var rec: jdk.jfr.Recording? = null
    val totals = mutableListOf<Long>()
    val setups = mutableListOf<Long>()
    val processes = mutableListOf<Long>()
    // One output directory, emptied after each run, as Gradle's
    // build/generated/ksp is the same from run to run.
    val outBase = Files.createTempDirectory("ksp-bench").toFile()
    for (run in 1..runs) {
        if (run == jfrFrom) {
            rec = jdk.jfr.Recording()
            rec.enable("jdk.ExecutionSample").withPeriod(Duration.ofMillis(1))
            rec.enable("jdk.NativeMethodSample").withPeriod(Duration.ofMillis(1))
            rec.start()
        }
        val out = File(outBase, "out")
        val providers = ServiceLoader.load(SymbolProcessorProvider::class.java).map { TimedProvider(it) }
        if (providers.isEmpty()) {
            System.err.println("KspBench: no SymbolProcessorProvider on the class path")
            exitProcess(2)
        }
        val config = KSPJvmConfig.Builder().apply {
            moduleName = "main"
            sourceRoots = srcs
            javaSourceRoots = emptyList()
            commonSourceRoots = emptyList()
            libraries = libs
            friends = emptyList()
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
            processorOptions = emptyMap()
        }.build()
        val t = Times()
        times = t
        phase = "setup"
        t.start = System.nanoTime()
        val code = KotlinSymbolProcessing(config, providers, BenchLogger).execute()
        val end = System.nanoTime()
        phase = "idle"
        val files = out.walkTopDown().filter { it.isFile && !it.path.contains("/caches/") }.count()
        out.deleteRecursively()
        val process = t.rounds.sumOf { it.second - it.first }
        totals += end - t.start
        setups += t.create - t.start
        processes += process
        println(
            "run $run $backend $code files=$files total=${ms(end - t.start)} setup=${ms(t.create - t.start)} " +
                "process=${ms(process)} rounds=${t.rounds.size} between=${ms(t.finishStart - t.create - process)} " +
                "finish=${ms(t.finishEnd - t.finishStart)} teardown=${ms(end - t.finishEnd)}",
        )
        if (code != KotlinSymbolProcessing.ExitCode.OK || BenchLogger.errors > 0) {
            System.err.println("KspBench: KSP exited with $code")
            exitProcess(1)
        }
    }
    outBase.deleteRecursively()
    rec?.let {
        it.stop()
        it.dump(File("bench.jfr").toPath())
    }
    fun median(l: List<Long>) = l.drop(l.size / 2).sorted().let { ms(it[it.size / 2]) }
    println(
        "summary $backend runs=$runs cold=${ms(totals[0])} warm=${median(totals)} " +
            "warm-setup=${median(setups)} warm-process=${median(processes)} min=${ms(totals.min())}",
    )
    // Stock KSP leaves non-daemon threads behind.
    exitProcess(0)
}
