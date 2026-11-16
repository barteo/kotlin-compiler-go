package io.github.barteo.kc.detekt

import io.github.barteo.kc.KcNative
import io.github.barteo.kc.Wire

/**
 * The calls into libkc's linter operations (Go's internal/lintops) and the
 * facts they return. The layouts are documented in internal/lintops.
 */
internal object KcLintOps {
    private const val ANALYZE = 20
    private const val FILE = 21
    private const val CLOSE = 22

    class Config(
        val jdkHome: String?,
        val moduleName: String,
        val jobs: Int,
        val classPath: List<String>,
        val friendPaths: List<String>,
        val optIns: List<String> = emptyList(),
    )

    class Diagnostic(val path: String, val line: Int, val column: Int, val code: String, val message: String) {
        override fun toString() = "$path:$line:$column: $message ($code)"
    }

    /** A file as kc parsed it: its number of psi nodes and the hash of their kinds. */
    class FileShape(val nodes: Int, val kinds: Long)

    class Result(val handle: Long, val diagnostics: List<Diagnostic>, val files: List<FileShape>)

    fun analyze(cfg: Config, paths: List<String>): Result {
        val w = Wire.Writer()
            // Without a JDK home, kc looks for one as it does: JAVA_HOME,
            // or the JDK of java on PATH.
            .writeString(cfg.jdkHome ?: "")
            .writeBool(false)
            .writeString(cfg.moduleName)
            .writeUint(cfg.jobs.toLong())
            .writeUint(cfg.classPath.size.toLong())
        cfg.classPath.forEach { w.writeString(it) }
        w.writeUint(cfg.friendPaths.size.toLong())
        cfg.friendPaths.forEach { w.writeString(it) }
        w.writeUint(cfg.optIns.size.toLong())
        cfg.optIns.forEach { w.writeString(it) }
        w.writeUint(paths.size.toLong())
        paths.forEach { w.writeString(it) }
        val r = Wire.Reader(KcNative.call(ANALYZE, w.message()))
        val handle = r.readInt()
        val diags = List(r.readUint().toInt()) {
            Diagnostic(r.readString(), r.readUint().toInt(), r.readUint().toInt(), r.readString(), r.readString())
        }
        val files = List(r.readUint().toInt()) { FileShape(r.readUint().toInt(), r.readUint()) }
        return Result(handle, diags, files)
    }

    fun file(handle: Long, index: Int, nodes: Int): KcFileFacts =
        KcFileFacts.read(Wire.Reader(KcNative.call(FILE, Wire.Writer().writeInt(handle).writeUint(index.toLong()).message())), nodes)

    fun close(handle: Long) {
        KcNative.call(CLOSE, Wire.Writer().writeInt(handle).message())
    }
}

/** A psi node of an analyzed file: the file's index and the node's ID. */
internal data class KcElem(val file: Int, val node: Int)

/** A type as kc resolved it (Go's analysis.Type). */
internal class KcType(
    val name: String,
    /** The Kotlin class id, a/b/C.D; "" for a type parameter. */
    val classId: String,
    val flags: Int,
    /** The declaration of a type parameter's owner, if it is in the sources. */
    val owner: KcElem?,
    /** 0 none, 1 in, 2 out, 3 star. */
    val projection: Int,
    val args: List<KcType>,
    val supertypes: List<String>,
) {
    val nullable get() = flags and 1 != 0
    val markedNullable get() = flags and 2 != 0
    val platform get() = flags and 4 != 0
    val parameter get() = flags and 8 != 0
    val sam get() = flags and 16 != 0
    val suspend get() = flags and 32 != 0
    val star get() = projection == 3

    override fun toString() = name + (if (args.isEmpty()) "" else args.joinToString(", ", "<", ">")) + if (markedNullable) "?" else if (platform) "!" else ""
}

/** A declaration (Go's analysis.Symbol). */
internal class KcSym(
    val id: Int,
    /** 1 class, 2 function, 3 constructor, 4 property, 5 local, 6 parameter. */
    val kind: Int,
    val name: String,
    val qualifiedName: String,
    val owner: String,
    val type: KcType?,
    val declaration: KcElem?,
    val supertypes: List<String>,
    val flags: Int,
) {
    val shadowed get() = flags and 1 != 0
    val hasAbstractMembers get() = flags and 2 != 0
    val hasConcreteMembers get() = flags and 4 != 0
    val hasAbstractSuperclass get() = flags and 8 != 0

    companion object {
        const val CLASS = 1
        const val FUNCTION = 2
        const val CONSTRUCTOR = 3
        const val PROPERTY = 4
        const val LOCAL = 5
        const val PARAMETER = 6
    }
}

internal class KcParam(val type: KcType, val name: String, val flags: Int) {
    val vararg get() = flags and 1 != 0
    val crossinline get() = flags and 2 != 0
    val noinline get() = flags and 4 != 0
}

/** A resolved call (Go's analysis.Call). */
internal class KcCall(
    val name: String,
    val ownerClassId: String,
    val declaration: KcElem?,
    val flags: Int,
    val declaredReceiver: KcType?,
    val receiver: KcType?,
    val implicitReceiver: KcElem?,
    val params: List<KcParam>,
    val argumentMapping: IntArray,
    val returnType: KcType?,
    val declaredReturnType: KcType?,
    val typeParameters: Int,
    val typeParameterNames: List<String>,
    val annotations: List<String>,
    val ownerAnnotations: List<String>,
    val packageAnnotations: List<String>,
    val targets: List<KcCall>,
) {
    val extension get() = flags and 1 != 0
    val suspend get() = flags and 2 != 0
    val inline get() = flags and 4 != 0
    val operator get() = flags and 8 != 0
    val fromJava get() = flags and 16 != 0
    val stableNames get() = flags and 32 != 0
    val valueInvocation get() = flags and 64 != 0
    val isConstructor get() = name.endsWith(".<init>")

    /** The callee's simple name: <init> for a constructor. */
    val simpleName get() = name.substringAfterLast('.')

    override fun toString() = name
}

internal class KcWhen(val exhaustive: Boolean, val category: String, val missing: List<String>)

/** What kc knows of a psi node (Go's analysis.Fact). */
internal class KcFact(
    val node: Int,
    /** 0 none, 1 declaration, 2 reference, 3 expression. */
    val role: Int,
    val type: KcType?,
    val symbol: Int,
    val call: KcCall?,
    val builtin: String?,
    val importableName: String?,
    val pkg: String?,
    val whenInfo: KcWhen?,
    val nonNullLeftSubtypeRight: Boolean?,
    val lambdaParameters: IntArray?,
    val returnTarget: KcElem?,
    val receiverTarget: KcElem?,
    val implicitReceiver: KcElem?,
    val delegateType: KcType?,
    val delegatedCalls: List<KcCall>,
    val accessors: List<KcCall>,
    val deprecated: String?,
    val flags: Int,
) {
    val classifier get() = flags and 1 != 0
    val flowNonNull get() = flags and 2 != 0
    val definitelyNonNull get() = flags and 4 != 0
    val impossibleCast get() = flags and 8 != 0
    val usedAsExpression get() = flags and 16 != 0
    val unreachable get() = flags and 32 != 0
    val suspendOperation get() = flags and 64 != 0
    val implicitParameter get() = flags and 128 != 0
}

/** A file's facts, by node, and the symbols they name. */
internal class KcFileFacts(val facts: Array<KcFact?>, val symbols: List<KcSym>) {
    operator fun get(node: Int): KcFact? = facts.getOrNull(node)

    companion object {
        fun read(r: Wire.Reader, nodes: Int): KcFileFacts {
            val types = ArrayList<KcType>()
            repeat(r.readUint().toInt()) {
                val name = r.readString()
                val classId = r.readString()
                val flags = r.readUint().toInt()
                val owner = elem(r)
                val projection = r.readUint().toInt()
                val args = List(r.readUint().toInt()) { types[r.readUint().toInt()] }
                types += KcType(name, classId, flags, owner, projection, args, strings(r))
            }
            fun opt(): KcType? = r.readUint().toInt().let { if (it == 0) null else types[it - 1] }
            val syms = List(r.readUint().toInt()) {
                KcSym(
                    id = r.readUint().toInt(), kind = r.readUint().toInt(), name = r.readString(),
                    qualifiedName = r.readString(), owner = r.readString(), type = opt(), declaration = elem(r),
                    supertypes = strings(r), flags = r.readUint().toInt(),
                )
            }
            fun call(): KcCall {
                val name = r.readString()
                val ownerClassId = r.readString()
                val decl = elem(r)
                val flags = r.readUint().toInt()
                val declaredReceiver = opt()
                val receiver = opt()
                val implicitReceiver = elem(r)
                val params = List(r.readUint().toInt()) { KcParam(types[r.readUint().toInt()], r.readString(), r.readUint().toInt()) }
                val mapping = IntArray(r.readUint().toInt()) { r.readInt().toInt() }
                val ret = opt()
                val declaredRet = opt()
                val tps = r.readUint().toInt()
                val tpNames = strings(r)
                val annots = strings(r)
                val ownerAnnots = strings(r)
                val pkgAnnots = strings(r)
                val targets = List(r.readUint().toInt()) { call() }
                return KcCall(name, ownerClassId, decl, flags, declaredReceiver, receiver, implicitReceiver, params, mapping,
                    ret, declaredRet, tps, tpNames, annots, ownerAnnots, pkgAnnots, targets)
            }
            val facts = arrayOfNulls<KcFact>(nodes + 1)
            repeat(r.readUint().toInt()) {
                val node = r.readUint().toInt()
                val mask = r.readUint().toInt()
                fun has(bit: Int) = mask and (1 shl bit) != 0
                val role = if (has(0)) r.readUint().toInt() else 0
                val type = if (has(1)) types[r.readUint().toInt()] else null
                val symbol = if (has(2)) r.readUint().toInt() else 0
                val call = if (has(3)) call() else null
                val builtin = if (has(4)) r.readString() else null
                val importable = if (has(5)) r.readString() else null
                val pkg = if (has(6)) r.readString() else null
                val whenInfo = if (has(7)) KcWhen(r.readBool(), r.readString(), strings(r)) else null
                val elvis = if (has(8)) r.readBool() else null
                val lambda = if (has(9)) IntArray(r.readUint().toInt()) { r.readUint().toInt() } else null
                val ret = if (has(10)) elem(r) else null
                val recv = if (has(11)) elem(r) else null
                val implicit = if (has(12)) elem(r) else null
                val delegate = if (has(13)) types[r.readUint().toInt()] else null
                val delegated = if (has(14)) List(r.readUint().toInt()) { call() } else emptyList()
                val accessors = if (has(15)) List(r.readUint().toInt()) { call() } else emptyList()
                val deprecated = if (has(16)) r.readString() else null
                val flags = if (has(17)) r.readUint().toInt() else 0
                val f = KcFact(node, role, type, symbol, call, builtin, importable, pkg, whenInfo, elvis, lambda, ret, recv,
                    implicit, delegate, delegated, accessors, deprecated, flags)
                if (node in facts.indices) facts[node] = f
            }
            return KcFileFacts(facts, syms)
        }

        private fun elem(r: Wire.Reader): KcElem? {
            val f = r.readUint().toInt()
            return if (f == 0) null else KcElem(f - 1, r.readUint().toInt())
        }

        private fun strings(r: Wire.Reader): List<String> {
            val n = r.readUint().toInt()
            return if (n == 0) emptyList() else List(n) { r.readString() }
        }
    }
}
