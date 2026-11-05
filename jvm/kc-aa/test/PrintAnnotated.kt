// PrintAnnotated runs KSP's processing loop, on a chosen backend, with a
// processor that prints the symbols annotated with the given annotations.
//
//	PrintAnnotated SRC_DIR OUT_DIR ANNOTATION... (-Dksp.backend=kc|aa)
//
// Each line is "annotation: kind name", sorted, followed by the
// annotation's arguments, defaults included, if it has any. An argument
// decls:a.b.C names a class whose origin and declarations it prints
// instead, as "decls:a.b.C: - kind name".
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
                if (name.startsWith("decls:")) {
                    val c = resolver.getClassDeclarationByName(resolver.getKSNameFromString(name.removePrefix("decls:")))
                    if (c == null) {
                        out += "$name: not found"
                        continue
                    }
                    out += "$name: ${c.origin} ${c.classKind} ${describe(c)}"
                    c.declarations.forEach { out += "$name: - ${describe(it)}" }
                    continue
                }
                resolver.getSymbolsWithAnnotation(name).forEach { out += "$name: ${describe(it)}" + arguments(it, name) }
            }
            return emptyList()
        }
    }
}

fun describe(a: KSAnnotated): String = when (a) {
    is KSFile -> "file ${a.fileName}"
    is KSClassDeclaration -> "class ${a.qualifiedName?.asString()}" + typeParams(a.typeParameters) +
        a.superTypes.toList().let { st -> if (st.isEmpty()) "" else st.joinToString(", ", " : ") { render(it) } }
    is KSFunctionDeclaration -> "function ${a.qualifiedName?.asString() ?: a.simpleName.asString()}" + typeParams(a.typeParameters) +
        a.parameters.joinToString(", ", "(", ")") { (if (it.isVararg) "vararg " else "") + "${it.name?.asString()}: ${render(it.type)}" } +
        (a.returnType?.let { ": " + render(it) } ?: "") + (a.extensionReceiver?.let { " on " + render(it) } ?: "")
    is KSPropertyGetter -> "getter ${a.receiver.simpleName.asString()}" + (a.returnType?.let { ": " + render(it) } ?: "")
    is KSPropertySetter -> "setter ${a.receiver.simpleName.asString()}"
    is KSPropertyDeclaration -> "property ${a.qualifiedName?.asString()}" +
        ": " + render(a.type) + (a.extensionReceiver?.let { " on " + render(it) } ?: "") +
        (if (a.isMutable) " (var)" else "")
    is KSValueParameter -> "parameter ${a.name?.asString()}: ${render(a.type)}"
    is KSTypeParameter -> "type parameter ${a.name.asString()}" + a.bounds.joinToString(", ", " : ") { render(it) }
    is KSTypeAlias -> "typealias ${a.qualifiedName?.asString()}"
    else -> "${a.javaClass.simpleName} $a"
}

/** The arguments of [a]'s annotations of class [name]: " (x = 1, ...)". */
fun arguments(a: KSAnnotated, name: String): String =
    a.annotations.filter { it.annotationType.resolve().declaration.qualifiedName?.asString() == name }
        .filter { it.arguments.isNotEmpty() }
        .joinToString("") { an -> an.arguments.joinToString(", ", " (", ")") { "${it.name?.asString()} = ${value(it.value)}" } }

/** An annotation argument's value as Kotlin writes it. */
fun value(v: Any?): String = when (v) {
    null -> "null"
    is String -> "\"$v\""
    is Char -> "'$v'"
    is Long -> "${v}L"
    is Float -> "${v}f"
    is Byte -> "$v.toByte()"
    is Short -> "$v.toShort()"
    is UByte, is UShort, is UInt -> "${v}u"
    is ULong -> "${v}uL"
    is KSType -> render(v) + "::class"
    is KSClassDeclaration -> v.qualifiedName?.asString() ?: v.simpleName.asString()
    is KSAnnotation -> "@" + (v.annotationType.resolve().declaration.qualifiedName?.asString() ?: v.shortName.asString()) +
        v.arguments.joinToString(", ", "(", ")") { "${it.name?.asString()} = ${value(it.value)}" }
    is List<*> -> v.joinToString(", ", "[", "]") { value(it) }
    else -> v.toString()
}

fun typeParams(tps: List<KSTypeParameter>) =
    if (tps.isEmpty()) "" else tps.joinToString(", ", "<", ">") {
        (if (it.variance == Variance.INVARIANT) "" else it.variance.label + " ") + it.name.asString() +
            it.bounds.joinToString("") { b -> " : " + render(b) }
    }

/** A resolved type as Kotlin writes it, with qualified names. */
fun render(ref: KSTypeReference): String =
    render(ref.resolve()) + (if (ref.element is KSDefNonNullReference) " & Any" else "")

fun render(t: KSType): String {
    if (t.isError) return "<ERROR ${t.declaration.simpleName.asString()}>"
    val d = t.declaration
    val name = if (d is KSTypeParameter) d.name.asString() else d.qualifiedName?.asString() ?: d.simpleName.asString()
    val args = if (t.arguments.isEmpty()) "" else t.arguments.joinToString(", ", "<", ">") { a ->
        when (a.variance) {
            Variance.STAR -> "*"
            Variance.INVARIANT -> render(a.type!!)
            else -> a.variance.label + " " + render(a.type!!)
        }
    }
    val fn = if (t.isFunctionType || t.isSuspendFunctionType) "fun " else ""
    return fn + name + args + (if (t.isMarkedNullable) "?" else "")
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
