// KspSuite runs KSP's own unit tests (kotlin-analysis-api's
// KSPUnitTestSuite, as AAConfiguredUnitTestSuite configures it) without
// Kotlin's test framework, on whichever KSP backend ksp.backend names, and
// writes one line per test to RESULTS.
//
//	KspSuite KSP_CHECKOUT WORK_DIR RESULTS COMPILER_CP [NAME_REGEX]
//
// It reads the test list from the suite's sources and each test's modules,
// files, processor and expected output from its test data file, as
// AbstractKSPTest does: library modules are compiled with K2JVMCompiler
// (from COMPILER_CP, in a class loader of their own, so its IntelliJ
// application doesn't meet KSP's) and javac, and the last module is
// processed by KotlinSymbolProcessing with the test's processor. System
// properties: ksptest.stdlib (the Kotlin standard library, a library of
// every module), ksptest.annotations (JetBrains' annotations jar, on
// javac's class path), ksptest.timeout (seconds per test, default 120).
//
// A RESULTS line is "STATUS name reason": PASS, FAIL (the output differs
// from the expected one; WORK_DIR/name.diff has both), ERROR (the run
// threw; WORK_DIR/name.log has the trace) or SKIP (@Disabled).
import com.google.devtools.ksp.impl.KotlinSymbolProcessing
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.google.devtools.ksp.processing.KSPLogger
import com.google.devtools.ksp.processor.AbstractTestProcessor
import com.google.devtools.ksp.symbol.KSNode
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.PrintStream
import java.io.PrintWriter
import java.io.StringWriter
import java.net.URLClassLoader
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import javax.tools.ToolProvider
import kotlin.system.exitProcess

enum class Kind { RUN, FAILING, THROWING }

class Case(val name: String, val path: String, val kind: Kind, val disabled: Boolean)

class SrcFile(val name: String, val text: String) {
    val isJava get() = name.endsWith(".java")
}

class Module(val name: String, val deps: List<String>, val friends: List<String>) {
    val files = mutableListOf<SrcFile>()
}

/** The tests of KSPUnitTestSuite, with AAConfiguredUnitTestSuite's overrides. */
fun collect(ksp: File): List<Case> {
    val dir = File(ksp, "kotlin-analysis-api/src/test/kotlin/com/google/devtools/ksp/test")
    val run = Regex("""(runTest|runFailingTest|runThrowingTest)\("\${'$'}AA_PATH/([^"]+)"\)""")
    fun read(f: File): LinkedHashMap<String, Case> {
        val cases = LinkedHashMap<String, Case>()
        val lines = f.readLines()
        for ((i, line) in lines.withIndex()) {
            val m = Regex("""^\s*(?:override )?fun (test\w+)\(\)\s*\{""").find(line) ?: continue
            val name = m.groupValues[1]
            var j = i - 1
            var disabled = false
            while (j >= 0 && lines[j].trim().startsWith("@")) {
                if (lines[j].trim() == "@Disabled" || lines[j].trim().startsWith("@Disabled(")) disabled = true
                j--
            }
            val body = lines.subList(i, minOf(i + 4, lines.size)).joinToString("\n")
            val r = run.find(body) ?: continue
            val kind = when (r.groupValues[1]) {
                "runFailingTest" -> Kind.FAILING
                "runThrowingTest" -> Kind.THROWING
                else -> Kind.RUN
            }
            cases[name] = Case(name, r.groupValues[2], kind, disabled)
        }
        return cases
    }
    val cases = read(File(dir, "KSPUnitTestSuite.kt"))
    cases.putAll(read(File(dir, "AAConfiguredUnitTestSuite.kt")))
    return cases.values.toList()
}

/**
 * The modules of a test file. Files keep their line numbers in it: each is
 * padded with an empty line for every line before its content, as Kotlin's
 * test framework does.
 */
fun modules(test: File): List<Module> {
    val lines = test.readLines()
    val mods = mutableListOf<Module>()
    var file: String? = null
    var start = 0
    fun flush(end: Int) {
        val f = file ?: return
        if (mods.isEmpty()) mods += Module("main", emptyList(), emptyList())
        val text = "\n".repeat(start) + lines.subList(start, end).joinToString("\n") + "\n"
        mods.last().files += SrcFile(f, text)
        file = null
    }
    val module = Regex("""^// MODULE:\s*([^(\s]+)(?:\(([^)]*)\))?(?:\(([^)]*)\))?""")
    for ((i, line) in lines.withIndex()) {
        val m = module.find(line)
        if (m != null) {
            flush(i)
            fun list(s: String) = s.split(',').map { it.trim() }.filter { it.isNotEmpty() }
            mods += Module(m.groupValues[1], list(m.groupValues[2]), list(m.groupValues[3]))
            continue
        }
        if (line.startsWith("// FILE:")) {
            flush(i)
            file = line.substringAfter("// FILE:").trim()
            start = i + 1
        }
    }
    flush(lines.size)
    if (mods.isEmpty() || mods.all { it.files.isEmpty() }) {
        mods.clear()
        mods += Module("main", emptyList(), emptyList()).also { it.files += SrcFile(test.name, lines.joinToString("\n") + "\n") }
    }
    return mods
}

fun expected(lines: List<String>): String =
    lines.dropWhile { !it.startsWith("// EXPECTED:") }.drop(1).takeWhile { !it.startsWith("// END") }
        .joinToString("\n") { it.substring(3).trim() }

class Logger(private val out: StringBuilder) : KSPLogger {
    override fun logging(message: String, symbol: KSNode?) {}
    override fun info(message: String, symbol: KSNode?) {}
    override fun warn(message: String, symbol: KSNode?) { out.appendLine("w: $message") }
    override fun error(message: String, symbol: KSNode?) { out.appendLine("e: $message") }
    override fun exception(e: Throwable) { out.appendLine("exception: ${trace(e)}") }
}

fun trace(e: Throwable): String = StringWriter().also { e.printStackTrace(PrintWriter(it)) }.toString()

class Suite(val work: File, compilerCp: List<File>) {
    val stdlib = System.getProperty("ksptest.stdlib")?.let(::File)
    val annotations = System.getProperty("ksptest.annotations")?.let(::File)
    private val compiler = URLClassLoader(compilerCp.map { it.toURI().toURL() }.toTypedArray(), ClassLoader.getPlatformClassLoader())
    private val k2jvm = compiler.loadClass("org.jetbrains.kotlin.cli.jvm.K2JVMCompiler")

    fun root(test: File, mod: String) = File(work, "${test.nameWithoutExtension}/$mod")

    fun write(dir: File, files: List<SrcFile>) {
        for (f in files) File(dir, f.name).apply { parentFile.mkdirs(); writeText(f.text) }
    }

    /** Compiles a library module into root/out, as AbstractKSPAATest.compileModule does. */
    fun compile(test: File, m: Module, log: StringBuilder) {
        val root = root(test, m.name)
        val kotlinSrc = File(root, "kotlinSrc").apply { mkdirs() }
        val javaSrc = File(root, "javaSrc").apply { mkdirs() }
        val out = File(root, "out").apply { mkdirs() }
        write(kotlinSrc, m.files.filter { !it.isJava })
        val javaFiles = m.files.filter { it.isJava }
        write(javaSrc, javaFiles)
        val deps = (m.deps + m.friends).map { File(root(test, it), "out") }
        val cp = deps.map { it.canonicalPath } + kotlinSrc.path + listOfNotNull(stdlib?.path)
        val args = arrayOf(kotlinSrc.path, javaSrc.path, "-d", out.absolutePath, "-no-stdlib", "-no-reflect",
            "-Xannotation-target-all", "-module-name", m.name, "-classpath", cp.joinToString(File.pathSeparator)) +
            (if (m.friends.isEmpty()) emptyArray() else arrayOf("-Xfriend-paths=" + deps.takeLast(m.friends.size).joinToString(",")))
        val buf = ByteArrayOutputStream()
        val inst = k2jvm.getDeclaredConstructor().newInstance()
        val old = Thread.currentThread().contextClassLoader
        Thread.currentThread().contextClassLoader = compiler
        try {
            k2jvm.getMethod("exec", PrintStream::class.java, Array<String>::class.java).invoke(inst, PrintStream(buf), args)
        } finally {
            Thread.currentThread().contextClassLoader = old
        }
        if (buf.size() > 0) log.appendLine("kotlinc ${m.name}:").append(buf.toString())
        if (javaFiles.isNotEmpty()) {
            val jcp = (deps + listOfNotNull(annotations, stdlib) + out).joinToString(File.pathSeparator) { it.absolutePath }
            val javac = ToolProvider.getSystemJavaCompiler()
            val jbuf = ByteArrayOutputStream()
            val files = javaFiles.map { File(javaSrc, it.name).path }
            javac.run(null, jbuf, jbuf, "-classpath", jcp, "-d", out.path, *files.toTypedArray())
            if (jbuf.size() > 0) log.appendLine("javac ${m.name}:").append(jbuf.toString())
        }
    }

    /** Runs a test and returns its output, as AbstractKSPTest.loadTest's lambda does. */
    fun run(test: File, log: StringBuilder): String {
        File(work, test.nameWithoutExtension).deleteRecursively()
        val lines = test.readLines()
        val mods = modules(test)
        val main = mods.last()
        for (lib in mods.dropLast(1)) compile(test, lib, log)
        val procName = lines.single { it.startsWith("// TEST PROCESSOR:") }.substringAfter(":").trim()
        val input = lines.find { it.startsWith("// PROCESSOR INPUT:") }?.substringAfter("// PROCESSOR INPUT:")
            ?.split(',')?.map { it.trim() }
        val cls = Class.forName("com.google.devtools.ksp.processor.$procName")
        val proc = (if (input == null) cls.getDeclaredConstructor().newInstance()
            else cls.getDeclaredConstructor(List::class.java).newInstance(input)) as AbstractTestProcessor
        val root = root(test, main.name)
        val kotlinSrc = File(root, "kotlinSrc").apply { mkdirs() }
        val javaSrc = File(root, "javaSrc")
        write(kotlinSrc, main.files.filter { !it.isJava })
        val javaFiles = main.files.filter { it.isJava }
        write(javaSrc, javaFiles)
        val kspTest = File(root, "kspTest")
        val config = KSPJvmConfig.Builder().apply {
            moduleName = main.name
            sourceRoots = listOf(kotlinSrc)
            javaSourceRoots = if (javaFiles.isEmpty()) emptyList() else listOf(javaSrc)
            commonSourceRoots = emptyList()
            jdkHome = File(System.getProperty("java.home"))
            jvmTarget = "1.8"
            languageVersion = System.getProperty("ksp.languageVersion", "2.4")
            apiVersion = languageVersion
            libraries = main.deps.map { File(root(test, it), "out") } + listOfNotNull(stdlib)
            friends = main.friends.map { File(root(test, it), "out") }
            projectBaseDir = root
            classOutputDir = File(kspTest, "classes/main")
            javaOutputDir = File(kspTest, "src/main/java")
            kotlinOutputDir = File(kspTest, "src/main/kotlin")
            resourceOutputDir = File(kspTest, "src/main/resources")
            cachesDir = File(kspTest, "kspCaches")
            outputBaseDir = kspTest
            incremental = true
        }.build()
        val code = KotlinSymbolProcessing(config, listOf(proc), Logger(log)).execute()
        val result = if (code != KotlinSymbolProcessing.ExitCode.OK) {
            listOf("KSP FAILED WITH EXIT CODE: ${code.name}") + proc.toResult()
        } else {
            proc.toResult()
        }
        return result.joinToString("\n")
    }
}

fun main(args: Array<String>) {
    if (args.size < 4) {
        System.err.println("usage: KspSuite KSP_CHECKOUT WORK_DIR RESULTS COMPILER_CP [NAME_REGEX]")
        exitProcess(2)
    }
    val ksp = File(args[0]).absoluteFile
    val work = File(args[1]).absoluteFile.apply { mkdirs() }
    val results = File(args[2])
    val compilerCp = args[3].split(File.pathSeparator).filter { it.isNotEmpty() }.map(::File)
    val only = args.getOrNull(4)?.let(::Regex)
    val timeout = System.getProperty("ksptest.timeout", "120").toLong()
    val cases = collect(ksp).filter { only == null || only.containsMatchIn(it.name) }
    System.err.println("ksp: ${cases.size} tests")
    val suite = Suite(work, compilerCp)
    val exec = Executors.newCachedThreadPool { r -> Thread(r).apply { isDaemon = true } }
    val counts = sortedMapOf<String, Int>()
    val lines = mutableListOf<String>()
    for (c in cases) {
        val test = File(ksp, "kotlin-analysis-api/../kotlin-analysis-api/testData/${c.path}").canonicalFile
        var status: String
        var reason = ""
        val log = StringBuilder()
        File(work, "${c.name}.diff").delete()
        File(work, "${c.name}.log").delete()
        if (c.disabled) {
            status = "SKIP"
            reason = "@Disabled"
        } else {
            val want = expected(test.readLines())
            val f = exec.submit<String> { suite.run(test, log) }
            try {
                val got = f.get(timeout, TimeUnit.SECONDS)
                val ok = when (c.kind) {
                    Kind.RUN -> got == want
                    Kind.FAILING -> got != want
                    Kind.THROWING -> false
                }
                status = if (ok) "PASS" else "FAIL"
                if (!ok) {
                    reason = if (c.kind == Kind.THROWING) "no InternalKSPException" else firstDiff(want, got)
                    File(work, "${c.name}.diff").writeText("--- want\n$want\n--- got\n$got\n")
                }
            } catch (e: TimeoutException) {
                f.cancel(true)
                status = "ERROR"
                reason = "timeout after ${timeout}s"
            } catch (e: ExecutionException) {
                val cause = e.cause ?: e
                if (c.kind == Kind.THROWING && cause.javaClass.simpleName == "InternalKSPException") {
                    status = "PASS"
                } else {
                    status = "ERROR"
                    reason = generateSequence(cause) { it.cause }.last().let { "${it.javaClass.simpleName}: ${it.message}" }
                        .lineSequence().first().take(200)
                    log.appendLine(trace(cause))
                }
            }
        }
        if (log.isNotEmpty()) File(work, "${c.name}.log").writeText(log.toString())
        counts.merge(status, 1, Int::plus)
        val line = "$status ${c.name} $reason".trimEnd()
        lines += line
        System.err.println(line)
    }
    results.writeText(lines.joinToString("\n", postfix = "\n"))
    val pass = counts["PASS"] ?: 0
    println("ksp: $pass/${cases.size} pass (${counts.entries.filter { it.key != "PASS" }.joinToString(", ") { "${it.value} ${it.key.lowercase()}" }})")
    exitProcess(0)
}

/** The first differing line of want and got, for the results line. */
fun firstDiff(want: String, got: String): String {
    val w = want.lines()
    val g = got.lines()
    for (i in 0 until maxOf(w.size, g.size)) {
        val a = w.getOrNull(i)
        val b = g.getOrNull(i)
        if (a != b) return "line ${i + 1}: want ${a?.take(80) ?: "<end>"} got ${b?.take(80) ?: "<end>"}"
    }
    return ""
}
