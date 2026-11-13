// Dump is a symbol processor that writes what KSP tells it about the
// sources to resources/dump.txt, for build.sh to compare between kc and
// stock KSP: each round's files and new files in KSP's order, then each new
// file's declarations, recursively, with their modifiers, origins,
// annotations and their arguments (defaults included), types (resolved,
// through type aliases and with replaced arguments), supertypes,
// getAllFunctions and getAllProperties (owner, origin, overridee,
// asMemberOf) and parameters. In round 1 it generates gen/Zeta.kt,
// gen/Alpha.kt and gen/Mid.kt, in that order, and from round 2 lists the
// package gen with getDeclarationsFromPackage. A symbol KSP throws on is
// written X:Exception.
@file:OptIn(com.google.devtools.ksp.KspExperimental::class)

import com.google.devtools.ksp.getAllSuperTypes
import com.google.devtools.ksp.getVisibility
import com.google.devtools.ksp.isAbstract
import com.google.devtools.ksp.isDefault
import com.google.devtools.ksp.processing.Dependencies
import com.google.devtools.ksp.processing.Resolver
import com.google.devtools.ksp.processing.SymbolProcessor
import com.google.devtools.ksp.processing.SymbolProcessorEnvironment
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.*

class DumpProvider : SymbolProcessorProvider {
    override fun create(environment: SymbolProcessorEnvironment) = Dump(environment)
}

class Dump(private val env: SymbolProcessorEnvironment) : SymbolProcessor {
    private val sb = StringBuilder()
    private var round = 0

    private fun line(depth: Int, s: String) {
        sb.append("  ".repeat(depth)).append(s).append('\n')
    }

    private fun safe(depth: Int, s: () -> String) = line(depth, runCatching(s).getOrElse { "X:" + it.javaClass.simpleName })

    private fun mods(m: Set<Modifier>) = m.map { it.name.lowercase() }.sorted().joinToString(" ")

    private fun type(t: KSType, depth: Int = 0): String {
        if (depth > 4) return "..."
        val d = t.declaration
        val kind = when (d) {
            is KSTypeAlias -> "alias"
            is KSClassDeclaration -> d.classKind.name.lowercase()
            is KSTypeParameter -> "tp"
            else -> d.javaClass.simpleName
        }
        val args = if (t.arguments.isEmpty()) "" else t.arguments.joinToString(",", "<", ">") { a ->
            a.variance.label + (a.type?.let { type(it.resolve(), depth + 1) } ?: "*")
        }
        val alias = if (d !is KSTypeAlias) "" else {
            val e = d.type.resolve()
            "{=" + type(e, depth + 1) + (if (t.arguments.isEmpty()) "" else " replaced=" + type(e.replace(t.arguments), depth + 1)) + "}"
        }
        val annots = t.annotations.joinToString("") { "@" + it.shortName.asString() + " " }
        val flags = listOfNotNull("?".takeIf { t.isMarkedNullable }, "!error".takeIf { t.isError },
            "!fun".takeIf { t.isFunctionType }, "!suspend".takeIf { t.isSuspendFunctionType }).joinToString("")
        return "$annots${d.qualifiedName?.asString() ?: d.simpleName.asString()}[$kind]$args$flags$alias /$t"
    }

    private fun ref(r: KSTypeReference?): String {
        if (r == null) return "null"
        val e = when (val el = r.element) {
            is KSClassifierReference -> "ref(${el.referencedName()})"
            null -> "null"
            else -> el.javaClass.simpleName
        }
        return "<$e ${r.annotations.joinToString("") { "@" + it.shortName.asString() + " " }}> " + type(r.resolve())
    }

    private fun value(x: Any?): String = when (x) {
        null -> "null"
        is KSType -> "type:" + type(x)
        is KSAnnotation -> "@" + x.shortName.asString() + x.arguments.joinToString(",", "(", ")") { "${it.name?.asString()}=${value(it.value)}" }
        is List<*> -> x.joinToString(",", "[", "]") { value(it) }
        is KSClassDeclaration -> "decl:" + x.qualifiedName?.asString() + "[${x.classKind}]"
        is String -> "\"$x\""
        else -> "${x.javaClass.simpleName}:$x"
    }

    private fun annotation(a: KSAnnotation, depth: Int) {
        safe(depth) { "@${a.shortName.asString()} type=${type(a.annotationType.resolve())} target=${a.useSiteTarget} origin=${a.origin}" }
        for (arg in a.arguments) safe(depth + 1) { "${arg.name?.asString()}=${value(arg.value)} default=${arg.isDefault()}" }
        for (arg in a.defaultArguments) safe(depth + 1) { "default ${arg.name?.asString()}=${value(arg.value)}" }
    }

    private fun declaration(d: KSDeclaration, depth: Int) {
        val kind = when (d) {
            is KSClassDeclaration -> "class"
            is KSFunctionDeclaration -> "fun"
            is KSPropertyDeclaration -> "prop"
            is KSTypeAlias -> "alias"
            else -> "decl"
        }
        safe(depth) {
            "$kind ${d.qualifiedName?.asString() ?: d.simpleName.asString()} [${mods(d.modifiers)}] ${d.getVisibility()} " +
                "origin=${d.origin} file=${d.containingFile?.fileName} parent=${d.parentDeclaration?.qualifiedName?.asString()} " +
                "expect=${d.isExpect} actual=${d.isActual}"
        }
        d.annotations.forEach { annotation(it, depth + 1) }
        d.typeParameters.forEach { tp ->
            safe(depth + 1) { "<${tp.name.asString()} ${tp.variance} reified=${tp.isReified} bounds=${tp.bounds.map(::ref).toList()}" }
        }
        when (d) {
            is KSClassDeclaration -> {
                safe(depth + 1) {
                    "kind=${d.classKind} companion=${d.isCompanionObject} abstract=${d.isAbstract()} " +
                        "primary=${d.primaryConstructor != null} type=${type(d.asStarProjectedType())}"
                }
                d.superTypes.forEach { s -> safe(depth + 1) { "super " + ref(s) } }
                safe(depth + 1) { "allSupers " + d.getAllSuperTypes().map { type(it) }.toList() }
                safe(depth + 1) { "sealed " + d.getSealedSubclasses().map { it.qualifiedName?.asString() }.toList() }
                d.declarations.forEach { declaration(it, depth + 1) }
                val self = d.asStarProjectedType()
                for (f in d.getAllFunctions()) safe(depth + 1) {
                    val m = runCatching { f.asMemberOf(self).let { m -> m.parameterTypes.map { it?.let(::type) }.toString() + " -> " + m.returnType?.let(::type) } }
                    "allFun ${f.qualifiedName?.asString()} [${mods(f.modifiers)}] abstract=${f.isAbstract} params=${f.parameters.map { it.name?.asString() }} " +
                        "parent=${f.parentDeclaration?.qualifiedName?.asString()} origin=${f.origin} overridee=${f.findOverridee()?.qualifiedName?.asString()} " +
                        "asMemberOf=${m.getOrElse { "X:" + it.javaClass.simpleName }}"
                }
                for (p in d.getAllProperties()) safe(depth + 1) {
                    "allProp ${p.qualifiedName?.asString()} [${mods(p.modifiers)}] abstract=${p.isAbstract()} parent=${p.parentDeclaration?.qualifiedName?.asString()} " +
                        "origin=${p.origin} overridee=${p.findOverridee()?.qualifiedName?.asString()} type=${ref(p.type)} " +
                        "asMemberOf=${runCatching { type(p.asMemberOf(self)) }.getOrElse { "X:" + it.javaClass.simpleName }}"
                }
            }
            is KSFunctionDeclaration -> {
                safe(depth + 1) {
                    "${d.functionKind} abstract=${d.isAbstract} receiver=${ref(d.extensionReceiver)} returns=${ref(d.returnType)} " +
                        "overridee=${d.findOverridee()?.qualifiedName?.asString()}"
                }
                d.parameters.forEach { p ->
                    safe(depth + 2) { "(${p.name?.asString()} default=${p.hasDefault} vararg=${p.isVararg} val=${p.isVal} var=${p.isVar} type=${ref(p.type)}" }
                    p.annotations.forEach { annotation(it, depth + 3) }
                }
            }
            is KSPropertyDeclaration -> safe(depth + 1) {
                "type=${ref(d.type)} mutable=${d.isMutable} delegated=${d.isDelegated()} field=${d.hasBackingField} receiver=${ref(d.extensionReceiver)} " +
                    "getter=${d.getter?.let { g -> mods(g.modifiers) + " " + g.annotations.map { it.shortName.asString() }.toList() }} " +
                    "setter=${d.setter?.let { s -> mods(s.modifiers) + " " + s.annotations.map { it.shortName.asString() }.toList() }}"
            }
            is KSTypeAlias -> safe(depth + 1) { "aliased=${ref(d.type)}" }
            else -> {}
        }
    }

    override fun process(resolver: Resolver): List<KSAnnotated> {
        round++
        line(0, "=== round $round")
        fun name(f: KSFile) = f.packageName.asString() + "/" + f.fileName
        line(0, "files " + resolver.getAllFiles().map(::name).toList())
        line(0, "new " + resolver.getNewFiles().map(::name).toList())
        line(0, "package gen " + resolver.getDeclarationsFromPackage("gen").map { it.qualifiedName?.asString() }.toList())
        if (round == 1) {
            for (n in listOf("Zeta", "Alpha", "Mid")) {
                env.codeGenerator.createNewFile(Dependencies(true), "gen", n).use {
                    it.write("package gen\n\nclass $n\n\nval p$n = 1\n\nfun f$n() {}\n".toByteArray())
                }
            }
        }
        resolver.getNewFiles().sortedBy(::name).forEach { f ->
            line(0, "file ${name(f)} origin=${f.origin}")
            f.annotations.forEach { annotation(it, 1) }
            f.declarations.forEach { declaration(it, 1) }
        }
        return emptyList()
    }

    override fun finish() {
        env.codeGenerator.createNewFileByPath(Dependencies(false), "dump", "txt").use { it.write(sb.toString().toByteArray()) }
    }
}
