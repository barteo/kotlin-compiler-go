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
// ksp.languageVersion (default 2.4; stock KSP 2.3.x's needs 2.3),
// jfr.from=N, which records execution samples from run N on into
// bench.jfr (see Classify.java), and bench.dump=FILE, which writes what
// the processors can see to FILE (DumpProcessor; not for timing).
//
// bench.edit=PATH times edits, as a daemon's incremental runs after an
// edit: the sources are copied, incremental processing is on and the
// output and caches directories are kept from run to run, and before each
// run after the first, the copy of PATH (relative to the first source
// directory, or the first Kotlin file in it if PATH is "auto") is edited:
// a body edit and a signature edit in turn, of a function appended to it.
// Each run's line has KSP's dirty files (dirty=, the first round's
// Resolver.getNewFiles).
//
// It is built against the KSP fork (jvm/kc-aa/build.sh) and against stock
// KSP; bench/ksp/run.sh runs both.
import com.google.devtools.ksp.impl.KotlinSymbolProcessing
import com.google.devtools.ksp.isLocal
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.google.devtools.ksp.processing.KSPLogger
import com.google.devtools.ksp.processing.Resolver
import com.google.devtools.ksp.processing.SymbolProcessor
import com.google.devtools.ksp.processing.SymbolProcessorEnvironment
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.*
import com.google.devtools.ksp.visitor.KSTopDownVisitor
import java.io.File
import java.nio.file.Files
import java.time.Duration
import java.util.ServiceLoader
import kotlin.system.exitProcess

/** The phase the main thread is in; Classify reads it from the stack instead. */
@Volatile
var phase = "setup"

class Times {
    var dirty = -1
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
        if (times.rounds.isEmpty() && edit) times.dirty = resolver.getNewFiles().count()
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

/**
 * With -Dbench.dump=FILE, DumpProcessor runs beside the processors and
 * writes, for every node a KSTopDownVisitor reaches from getAllFiles, what
 * the exhaustive processor asks of it: kind, origin, location, names,
 * modifiers, resolved types, overridees and argument values, one line per
 * node, sorted. Two backends that agree write the same file. It is for
 * conformance runs only, never for timing.
 */
class DumpProvider(private val file: File) : SymbolProcessorProvider {
    override fun create(environment: SymbolProcessorEnvironment): SymbolProcessor = DumpProcessor(file)
}

class DumpProcessor(private val file: File) : SymbolProcessor {
    private val lines = mutableListOf<String>()

    override fun process(resolver: Resolver): List<KSAnnotated> {
        val v = DumpVisitor(lines)
        resolver.getAllFiles().forEach { it.accept(v, Unit) }
        return emptyList()
    }

    override fun finish() {
        lines.sort()
        file.writeText(lines.joinToString("\n", postfix = "\n"))
    }
}

private fun loc(n: KSNode): String = when (val l = runCatching { n.location }.getOrElse { return "!throws" }) {
    is FileLocation -> "${File(l.filePath).name}:${l.lineNumber}"
    else -> "-"
}

private fun ty(t: KSType): String =
    (if (t.isError) "!" else "") + t.toString() + "/" + (t.declaration.qualifiedName?.asString() ?: t.declaration.simpleName.asString())

private fun qn(d: KSDeclaration?): String = d?.let { it.qualifiedName?.asString() ?: ("<" + it.simpleName.asString() + ">") } ?: "-"

private fun value(v: Any?): String = when (v) {
    null -> "null"
    is KSType -> ty(v)
    is KSDeclaration -> qn(v)
    is KSAnnotation -> "@" + v.shortName.asString()
    is List<*> -> v.joinToString(",", "[", "]") { value(it) }
    is Array<*> -> v.joinToString(",", "[", "]") { value(it) }
    else -> v.toString()
}

private fun safe(f: () -> Any?): String = try {
    f().toString()
} catch (e: Exception) {
    "!throws"
}

class DumpVisitor(private val lines: MutableList<String>) : KSTopDownVisitor<Unit, Unit>() {
    override fun defaultHandler(node: KSNode, data: Unit) {}

    // An exception KSP throws is part of what a processor sees: it is
    // written, as !throws, in place of the description. (Its class isn't:
    // KSP 2.3.6 and the 2.3.9 fork throw different ones for the same thing.)
    private fun emit(node: KSNode, kind: String, what: () -> String) {
        val w = try {
            what()
        } catch (e: Exception) {
            "!throws"
        }
        lines += "${loc(node)} $kind ${node.origin} $w"
    }

    override fun visitAnnotation(annotation: KSAnnotation, data: Unit) {
        emit(annotation, "anno") { "${annotation.shortName.asString()} ${annotation.useSiteTarget}" }
        super.visitAnnotation(annotation, data)
    }

    override fun visitClassDeclaration(classDeclaration: KSClassDeclaration, data: Unit) {
        val c = classDeclaration
        emit(c, "class") { "${qn(c)} ${c.classKind} companion=${c.isCompanionObject} ${c.modifiers.sorted()} " +
            "parent=${qn(c.parentDeclaration)} star=${ty(c.asStarProjectedType())} ctor=${c.primaryConstructor != null}" }
        super.visitClassDeclaration(classDeclaration, data)
    }

    override fun visitClassifierReference(reference: KSClassifierReference, data: Unit) {
        emit(reference, "ref") { reference.referencedName() }
        super.visitClassifierReference(reference, data)
    }

    override fun visitFile(file: KSFile, data: Unit) {
        lines += "${file.fileName} file ${file.packageName.asString()}"
        super.visitFile(file, data)
    }

    override fun visitFunctionDeclaration(function: KSFunctionDeclaration, data: Unit) {
        val f = function
        emit(f, "fun") { "${qn(f)} ${safe { f.functionKind }} abstract=${f.isAbstract} ${f.modifiers.sorted()} " +
            "parent=${qn(f.parentDeclaration)} overridee=${safe { qn(f.findOverridee()) }}" }
        super.visitFunctionDeclaration(function, data)
    }

    override fun visitPropertyAccessor(accessor: KSPropertyAccessor, data: Unit) {
        emit(accessor, "accessor") { "${qn(accessor.receiver)} ${accessor.modifiers.sorted()}" }
        super.visitPropertyAccessor(accessor, data)
    }

    override fun visitPropertyDeclaration(property: KSPropertyDeclaration, data: Unit) {
        val p = property
        emit(p, "prop") {
            val extra = if (!p.isLocal()) " mutable=${p.isMutable} delegated=${p.isDelegated()} overridee=${safe { qn(p.findOverridee()) }}" else " local"
            "${qn(p)} ${p.modifiers.sorted()} parent=${qn(p.parentDeclaration)}$extra"
        }
        super.visitPropertyDeclaration(property, data)
    }

    override fun visitTypeAlias(typeAlias: KSTypeAlias, data: Unit) {
        emit(typeAlias, "alias") { "${qn(typeAlias)} ${typeAlias.name.asString()}" }
        super.visitTypeAlias(typeAlias, data)
    }

    override fun visitTypeArgument(typeArgument: KSTypeArgument, data: Unit) {
        emit(typeArgument, "targ") { typeArgument.variance.toString() }
        super.visitTypeArgument(typeArgument, data)
    }

    override fun visitTypeParameter(typeParameter: KSTypeParameter, data: Unit) {
        val t = typeParameter
        emit(t, "tparam") { "${t.name.asString()} ${t.variance} reified=${t.isReified}" }
        super.visitTypeParameter(typeParameter, data)
    }

    override fun visitTypeReference(typeReference: KSTypeReference, data: Unit) {
        emit(typeReference, "type") { ty(typeReference.resolve()) }
        super.visitTypeReference(typeReference, data)
    }

    override fun visitValueArgument(valueArgument: KSValueArgument, data: Unit) {
        val a = valueArgument
        emit(a, "arg") { "${a.name?.asString()} spread=${a.isSpread} ${value(a.value)}" }
        super.visitValueArgument(valueArgument, data)
    }

    override fun visitValueParameter(valueParameter: KSValueParameter, data: Unit) {
        val p = valueParameter
        emit(p, "param") { "${p.name?.asString()} crossinline=${p.isCrossInline} noinline=${p.isNoInline} " +
            "val=${p.isVal} var=${p.isVar} vararg=${p.isVararg} default=${p.hasDefault}" }
        super.visitValueParameter(valueParameter, data)
    }
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

/** Whether runs edit a source (bench.edit). */
val edit = System.getProperty("bench.edit") != null

/**
 * Edits [file] for run [run] (2, 3, …): [base], its text, with a function
 * appended whose signature changes on even runs (run 2 adds it) and whose
 * body only changes on odd ones.
 */
private fun applyEdit(file: File, base: String, run: Int) {
    val type = if (run / 2 % 2 == 1) "Int" else "Any"
    file.writeText(base + "\nfun kspBenchEdit(): $type {\n    return $run\n}\n")
}

fun main(args: Array<String>) {
    if (args.size != 3) {
        System.err.println("usage: KspBench SRC_DIR[:SRC_DIR...] LIBS_FILE RUNS")
        exitProcess(2)
    }
    var srcs = args[0].split(File.pathSeparator).filter { it.isNotEmpty() }.map { File(it).absoluteFile }
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
    var editFile: File? = null
    var editBase = ""
    if (edit) {
        srcs = srcs.mapIndexed { i, d -> File(outBase, "src$i").also { d.copyRecursively(it) } }
        val path = System.getProperty("bench.edit")
        editFile = if (path == "auto") srcs[0].walkTopDown().filter { it.isFile && it.extension == "kt" }.minBy { it.path }
        else File(srcs[0], path)
        editBase = editFile.readText()
        println("edit ${editFile.relativeTo(srcs[0])}")
    }
    for (run in 1..runs) {
        if (run > 1) editFile?.let { applyEdit(it, editBase, run) }
        if (run == jfrFrom) {
            rec = jdk.jfr.Recording()
            rec.enable("jdk.ExecutionSample").withPeriod(Duration.ofMillis(1))
            rec.enable("jdk.NativeMethodSample").withPeriod(Duration.ofMillis(1))
            rec.enable("jdk.GarbageCollection")
            rec.enable("jdk.SafepointBegin")
            rec.start()
        }
        val out = File(outBase, "out")
        val providers = ServiceLoader.load(SymbolProcessorProvider::class.java).map { TimedProvider(it) } +
            listOfNotNull(System.getProperty("bench.dump")?.let { DumpProvider(File(it)) })
        if (providers.isEmpty()) {
            System.err.println("KspBench: no SymbolProcessorProvider on the class path")
            exitProcess(2)
        }
        val config = KSPJvmConfig.Builder().apply {
            moduleName = "main"
            sourceRoots = srcs
            javaSourceRoots = srcs // as KSP's Gradle plugin passes them: kc reads Java files from these only
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
            incremental = edit
            // As Gradle passes them: KSP doesn't look for changes itself.
            modifiedSources = listOfNotNull(editFile?.takeIf { run > 1 })
            processorOptions = emptyMap()
        }.build()
        val t = Times()
        times = t
        phase = "setup"
        t.start = System.nanoTime()
        val code = try {
            KotlinSymbolProcessing(config, providers, BenchLogger).execute()
        } catch (e: Throwable) {
            // KSP leaves non-daemon threads behind, which would keep the JVM.
            e.printStackTrace()
            exitProcess(1)
        }
        val end = System.nanoTime()
        phase = "idle"
        val files = out.walkTopDown().filter { it.isFile && !it.path.contains("/caches/") }.count()
        if (!edit) out.deleteRecursively()
        val process = t.rounds.sumOf { it.second - it.first }
        totals += end - t.start
        setups += t.create - t.start
        processes += process
        println(
            "run $run $backend $code files=$files total=${ms(end - t.start)} setup=${ms(t.create - t.start)} " +
                "process=${ms(process)} rounds=${t.rounds.size} between=${ms(t.finishStart - t.create - process)} " +
                "finish=${ms(t.finishEnd - t.finishStart)} teardown=${ms(end - t.finishEnd)}" +
                (if (edit) " dirty=${t.dirty}" else ""),
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
    // Edits: the medians of the body edits' runs (3, 5, …) and the
    // signature edits' (4, 6, …), all but run 2.
    fun edits(odd: Boolean) = totals.filterIndexed { i, _ -> i >= 2 && (i % 2 == 0) == odd }
        .sorted().let { if (it.isEmpty()) "-" else ms(it[it.size / 2]).toString() }
    println(
        "summary $backend runs=$runs cold=${ms(totals[0])} warm=${median(totals)} " +
            "warm-setup=${median(setups)} warm-process=${median(processes)} min=${ms(totals.min())}" +
            // KSP's BenchRunner: Hot is the fastest of 3 runs after 3 warm-ups.
            (if (runs >= 6) " hot=${ms(totals.subList(3, 6).min())}" else "") +
            (if (edit) " edit-body=${edits(true)} edit-signature=${edits(false)}" else ""),
    )
    // Stock KSP leaves non-daemon threads behind.
    exitProcess(0)
}
