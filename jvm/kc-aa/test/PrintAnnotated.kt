// PrintAnnotated runs KSP's processing loop, on a chosen backend, with a
// processor that prints the symbols annotated with the given annotations.
//
//	PrintAnnotated SRC_DIR OUT_DIR ANNOTATION... (-Dksp.backend=kc|aa)
//
// Each line is "annotation: kind name", sorted.
import com.google.devtools.ksp.impl.KotlinSymbolProcessing
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.google.devtools.ksp.processing.KSPLogger
import com.google.devtools.ksp.processing.Resolver
import com.google.devtools.ksp.processing.SymbolProcessor
import com.google.devtools.ksp.processing.SymbolProcessorEnvironment
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.*
import java.io.File
import kotlin.system.exitProcess

class PrintAnnotatedProvider(private val names: List<String>, private val out: MutableList<String>) :
    SymbolProcessorProvider {
    override fun create(environment: SymbolProcessorEnvironment) = object : SymbolProcessor {
        override fun process(resolver: Resolver): List<KSAnnotated> {
            for (name in names) {
                resolver.getSymbolsWithAnnotation(name).forEach { out += "$name: ${describe(it)}" }
            }
            return emptyList()
        }
    }
}

fun describe(a: KSAnnotated): String = when (a) {
    is KSFile -> "file ${a.fileName}"
    is KSClassDeclaration -> "class ${a.qualifiedName?.asString()}"
    is KSFunctionDeclaration -> "function ${a.qualifiedName?.asString()}"
    is KSPropertyGetter -> "getter ${a.receiver.simpleName.asString()}"
    is KSPropertySetter -> "setter ${a.receiver.simpleName.asString()}"
    is KSPropertyDeclaration -> "property ${a.qualifiedName?.asString()}"
    is KSValueParameter -> "parameter ${a.name?.asString()}"
    is KSTypeParameter -> "type parameter ${a.name.asString()}"
    is KSTypeAlias -> "typealias ${a.qualifiedName?.asString()}"
    else -> "${a.javaClass.simpleName} $a"
}

object StderrLogger : KSPLogger {
    override fun logging(message: String, symbol: KSNode?) {}
    override fun info(message: String, symbol: KSNode?) {}
    override fun warn(message: String, symbol: KSNode?) = System.err.println("w: $message")
    override fun error(message: String, symbol: KSNode?) = System.err.println("e: $message")
    override fun exception(e: Throwable) {
        e.printStackTrace()
    }
}

fun main(args: Array<String>) {
    if (args.size < 3) {
        System.err.println("usage: PrintAnnotated SRC_DIR OUT_DIR ANNOTATION...")
        exitProcess(2)
    }
    val src = File(args[0]).absoluteFile
    val out = File(args[1]).absoluteFile
    val config = KSPJvmConfig.Builder().apply {
        moduleName = "slice"
        sourceRoots = listOf(src)
        javaSourceRoots = emptyList()
        commonSourceRoots = emptyList()
        libraries = System.getProperty("kc.stdlib")?.let { listOf(File(it)) } ?: emptyList()
        jdkHome = File(System.getProperty("java.home"))
        projectBaseDir = src
        outputBaseDir = out
        cachesDir = File(out, "caches")
        classOutputDir = File(out, "classes")
        kotlinOutputDir = File(out, "kotlin")
        javaOutputDir = File(out, "java")
        resourceOutputDir = File(out, "resources")
        languageVersion = "2.4"
        apiVersion = "2.4"
        jvmTarget = "17"
        jvmDefaultMode = "enable"
        incremental = false
    }.build()
    val lines = mutableListOf<String>()
    val code = KotlinSymbolProcessing(config, listOf(PrintAnnotatedProvider(args.drop(2), lines)), StderrLogger).execute()
    lines.sorted().forEach(::println)
    if (code != KotlinSymbolProcessing.ExitCode.OK) {
        System.err.println("KSP exited with $code")
        exitProcess(1)
    }
}
