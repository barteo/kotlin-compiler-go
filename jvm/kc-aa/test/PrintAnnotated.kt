// PrintAnnotated runs KSP's processing loop, on a chosen backend, with a
// processor that prints the symbols annotated with the given annotations.
//
//	PrintAnnotated SRC_DIR OUT_DIR ANNOTATION... (-Dksp.backend=kc|aa)
//
// Each line is "annotation: kind name", sorted, followed by the
// annotation's arguments, defaults included, if it has any. An argument
// decls:a.b.C names a class whose origin and declarations it prints
// instead, as "decls:a.b.C: - kind name". Other modes:
//
//	members:a.b.C[:n1,n2]  getAllFunctions and getAllProperties (named n1,
//	                       n2 if given): origin, parent, overridee, asMemberOf C
//	assign:a.b.C           C's properties' types paired, target then source:
//	                       target.isAssignableFrom(source)
//	sealed:a.b.C           getSealedSubclasses
//	overrides:a.b.C#f:x.Y#g  Resolver.overrides(C.f, Y.g)
//	package:a.b[:n1,n2]    getDeclarationsFromPackage (named n1, n2 if given),
//	                       with the owner JVM class of functions and properties
//	fun:a.b.f, prop:a.b.p  getFunctionDeclarationsByName(includeTopLevel),
//	                       getPropertyDeclarationByName(includeTopLevel)
//	alias:a.b.C            C's properties' types: the declaration each names,
//	                       a type alias's type, and the declaration it expands to
import com.google.devtools.ksp.KspExperimental
import com.google.devtools.ksp.getDeclaredFunctions
import com.google.devtools.ksp.getDeclaredProperties
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
                val mode = name.substringBefore(':', "")
                if (mode in setOf("members", "assign", "sealed", "overrides", "package", "fun", "prop", "alias")) {
                    mode(resolver, mode, name.removePrefix("$mode:"), name)
                    continue
                }
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

        private fun cls(resolver: Resolver, name: String) =
            resolver.getClassDeclarationByName(resolver.getKSNameFromString(name)) ?: error("$name: not found")

        @OptIn(KspExperimental::class)
        private fun owner(resolver: Resolver, d: KSDeclaration): String = when (d) {
            is KSFunctionDeclaration -> " [" + resolver.getOwnerJvmClassName(d) + "]"
            is KSPropertyDeclaration -> " [" + resolver.getOwnerJvmClassName(d) + "]"
            else -> ""
        }

        @OptIn(KspExperimental::class)
        private fun mode(resolver: Resolver, mode: String, arg: String, tag: String) {
            when (mode) {
                "package" -> {
                    val only = arg.substringAfter(':', "").split(',').filter { it.isNotEmpty() }.toSet()
                    resolver.getDeclarationsFromPackage(arg.substringBefore(':'))
                        .filter { only.isEmpty() || it.simpleName.asString() in only }
                        .map { "$tag: ${it.origin} ${describe(it)}" + owner(resolver, it) }
                        .sorted().forEach { out += it }
                }
                "fun" -> resolver.getFunctionDeclarationsByName(resolver.getKSNameFromString(arg), includeTopLevel = true)
                    .forEach { out += "$tag: ${it.origin} ${describe(it)}" + owner(resolver, it) }
                "prop" -> resolver.getPropertyDeclarationByName(resolver.getKSNameFromString(arg), includeTopLevel = true)
                    .let { out += if (it == null) "$tag: not found" else "$tag: ${it.origin} ${describe(it)}" + owner(resolver, it) }
                "alias" -> for (p in cls(resolver, arg).getDeclaredProperties()) {
                    val t = p.type.resolve()
                    val d = t.declaration
                    val expanded = if (d is KSTypeAlias) {
                        var e: KSDeclaration = d
                        while (e is KSTypeAlias) e = e.type.resolve().declaration
                        " = ${render(d.type)}; expands to ${e.qualifiedName?.asString()}"
                    } else ""
                    out += "$tag: ${p.simpleName.asString()}: ${render(t)} names ${describe(d)}$expanded"
                }
                "members" -> {
                    val c = cls(resolver, arg.substringBefore(':'))
                    val only = arg.substringAfter(':', "").split(',').filter { it.isNotEmpty() }.toSet()
                    val self = c.asStarProjectedType()
                    val members = c.getAllFunctions() + c.getAllProperties()
                    for (m in members) {
                        if (only.isNotEmpty() && m.simpleName.asString() !in only) continue
                        val overridee = when (m) {
                            is KSFunctionDeclaration -> m.findOverridee()
                            is KSPropertyDeclaration -> m.findOverridee()
                            else -> null
                        }?.let {
                            " overrides ${it.parentDeclaration?.qualifiedName?.asString()}.${it.simpleName.asString()}"
                        } ?: ""
                        val asMember = when (m) {
                            is KSFunctionDeclaration -> m.asMemberOf(self).let { f ->
                                f.parameterTypes.joinToString(", ", "(", ")") { it?.let(::render) ?: "?" } +
                                    ": " + (f.returnType?.let(::render) ?: "?")
                            }
                            is KSPropertyDeclaration -> ": " + render(m.asMemberOf(self))
                            else -> ""
                        }
                        out += "$tag: ${m.origin} ${m.simpleName.asString()} in " +
                            "${m.parentDeclaration?.qualifiedName?.asString()}$overridee; as member $asMember"
                    }
                }
                "assign" -> {
                    val ps = cls(resolver, arg).getDeclaredProperties().toList()
                    for (i in 0 until ps.size / 2) {
                        val to = ps[2 * i].type.resolve()
                        val from = ps[2 * i + 1].type.resolve()
                        out += "$tag: ${render(to)} <- ${render(from)}: ${to.isAssignableFrom(from)}"
                    }
                }
                "sealed" -> cls(resolver, arg).getSealedSubclasses().forEach { out += "$tag: ${it.qualifiedName?.asString()}" }
                "overrides" -> {
                    val (a, b) = arg.split(':').map { s ->
                        val c = cls(resolver, s.substringBefore('#'))
                        c.getDeclaredFunctions().first { it.simpleName.asString() == s.substringAfter('#') }
                    }
                    out += "$tag: ${resolver.overrides(a, b)}"
                }
            }
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
