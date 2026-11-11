// Members lists the Analysis API members the KSP fork uses and whether kc
// answers them, as jvm/kc-aa/members.txt (written by build.sh members).
//
//	java -cp ... MembersKt OUT KSP_JAR KC_AA_JAR
//
// The members are those KSP_JAR's bytecode refers to: method calls, field
// reads and method references whose owner is in
// org.jetbrains.kotlin.analysis.api. Each gets a status:
//
//	kc       kc answers it: a method of KcAnalysis that the KaSession
//	         proxy dispatches to, or an override in kc's classes
//	partial  some of kc's classes that implement the member's owner
//	         override it, and the others leave it to a generated stub
//	stub     kc's classes leave it to a generated stub, or the KaSession
//	         proxy has no method for it: a call fails with
//	         UnsupportedOperationException("kc: ...")
//	api      the Analysis API answers it itself: a default method, a
//	         concrete or static member, a constructor or a field, which
//	         may in turn call members listed here
//	none     an abstract member of a type kc has no class for
import org.jetbrains.org.objectweb.asm.ClassReader
import org.jetbrains.org.objectweb.asm.ClassVisitor
import org.jetbrains.org.objectweb.asm.Handle
import org.jetbrains.org.objectweb.asm.MethodVisitor
import org.jetbrains.org.objectweb.asm.Opcodes
import org.jetbrains.org.objectweb.asm.Type
import java.io.File
import java.lang.reflect.Method
import java.lang.reflect.Modifier
import java.util.zip.ZipFile

private const val API = "org/jetbrains/kotlin/analysis/api/"
private const val KC = "io.github.barteo.kc.aa."

private data class Ref(val owner: String, val name: String, val desc: String, val field: Boolean)

fun main(args: Array<String>) {
    val (out, kspJar, kcJar) = args
    val refs = sortedSetOf(compareBy<Ref>({ it.owner }, { it.name }, { it.desc }))
    ZipFile(kspJar).use { z ->
        for (e in z.entries()) {
            if (!e.name.endsWith(".class")) continue
            ClassReader(z.getInputStream(e).readBytes()).accept(object : ClassVisitor(Opcodes.ASM9) {
                override fun visitMethod(access: Int, name: String?, descriptor: String?, signature: String?, exceptions: Array<out String>?) =
                    object : MethodVisitor(Opcodes.ASM9) {
                        override fun visitMethodInsn(opcode: Int, owner: String, name: String, desc: String, itf: Boolean) {
                            if (owner.startsWith(API)) refs += Ref(owner, name, desc, false)
                        }

                        override fun visitFieldInsn(opcode: Int, owner: String, name: String, desc: String) {
                            if (owner.startsWith(API)) refs += Ref(owner, name, desc, true)
                        }

                        override fun visitInvokeDynamicInsn(name: String?, desc: String?, bsm: Handle?, vararg bsmArgs: Any?) {
                            for (a in bsmArgs) if (a is Handle && a.owner.startsWith(API)) refs += Ref(a.owner, a.name, a.desc, false)
                        }
                    }
            }, ClassReader.SKIP_FRAMES)
        }
    }

    val loader = Thread.currentThread().contextClassLoader
    val kcClasses = ZipFile(kcJar).use { z ->
        z.entries().asSequence().map { it.name }
            .filter { it.endsWith(".class") }
            .map { it.removeSuffix(".class").replace('/', '.') }
            .filter { it.startsWith(KC) }
            .mapNotNull { runCatching { Class.forName(it, false, loader) }.getOrNull() }
            .filter { !it.isInterface && !Modifier.isAbstract(it.modifiers) && !it.simpleName.startsWith("Stub") }
            .toList()
    }
    val session = Class.forName("org.jetbrains.kotlin.analysis.api.KaSession", false, loader)
    val analysis = Class.forName(KC + "KcAnalysis", false, loader)

    fun status(r: Ref): String {
        if (r.field || r.name == "<init>") return "api"
        val owner = runCatching { Class.forName(r.owner.replace('/', '.'), false, loader) }.getOrNull() ?: return "none"
        val m = (owner.methods.asSequence() + owner.declaredMethods.asSequence() +
            generateSequence(owner.superclass) { it.superclass }.flatMap { it.declaredMethods.asSequence() })
            .firstOrNull { it.name == r.name && Type.getMethodDescriptor(it) == r.desc } ?: return "none"
        if (Modifier.isStatic(m.modifiers)) return "api"
        if (owner.isAssignableFrom(session) && owner.isInterface) {
            val impl = runCatching { analysis.getMethod(m.name, *m.parameterTypes) }.getOrNull()
            return when {
                impl != null -> "kc"
                m.isDefault -> "api"
                else -> "stub"
            }
        }
        val impls = kcClasses.filter { owner.isAssignableFrom(it) }
        if (impls.isEmpty()) return if (Modifier.isAbstract(m.modifiers)) "none" else "api"
        val by = impls.map { c ->
            val d: Method = runCatching { c.getMethod(m.name, *m.parameterTypes) }.getOrNull() ?: return@map "none"
            val decl = d.declaringClass
            when {
                decl.name.startsWith(KC) && decl.simpleName.startsWith("Stub") -> "stub"
                decl.name.startsWith(KC) -> "kc"
                Modifier.isAbstract(d.modifiers) -> "stub"
                else -> "api"
            }
        }.toSet()
        return when {
            by == setOf("kc") -> "kc"
            "kc" in by -> "partial"
            "stub" in by -> "stub"
            else -> "api"
        }
    }

    fun group(owner: String): String = owner.removePrefix(API).substringBeforeLast('/', "").substringBefore('/').ifEmpty { "session" }
    fun simple(owner: String) = owner.substringAfterLast('/').replace('$', '.')
    fun params(desc: String) = Type.getArgumentTypes(desc).joinToString(", ") { it.className.substringAfterLast('.').replace('$', '.') }

    val lines = refs.map { r ->
        val member = if (r.field) "${simple(r.owner)}.${r.name}" else "${simple(r.owner)}.${r.name}(${params(r.desc)})"
        Triple(group(r.owner), status(r), member)
    }.sortedWith(compareBy({ it.first }, { it.third }))
    val counts = lines.groupingBy { it.second }.eachCount()
    val byGroup = lines.groupBy { it.first }
    File(out).bufferedWriter().use { w ->
        w.write("# The Analysis API members the KSP fork uses, and whether kc answers them.\n")
        w.write("# Generated by jvm/kc-aa/build.sh members (gen/Members.kt); see the statuses there.\n")
        w.write("#\n")
        w.write("# total ${lines.size}: " + listOf("kc", "partial", "stub", "api", "none").joinToString(", ") { "$it ${counts[it] ?: 0}" } + "\n")
        for ((g, ls) in byGroup) {
            val c = ls.groupingBy { it.second }.eachCount()
            w.write("# $g ${ls.size}: " + listOf("kc", "partial", "stub", "api", "none").filter { c[it] != null }.joinToString(", ") { "$it ${c[it]}" } + "\n")
        }
        for ((g, ls) in byGroup) {
            w.write("\n[$g]\n")
            for ((_, s, m) in ls) w.write("%-8s%s\n".format(s, m))
        }
    }
}
