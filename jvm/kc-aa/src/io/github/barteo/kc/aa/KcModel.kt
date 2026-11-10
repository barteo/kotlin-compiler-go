package io.github.barteo.kc.aa

import io.github.barteo.kc.KcNative
import io.github.barteo.kc.Wire

/**
 * The calls into libkc's Analysis API operations (Go's internal/kaops) and
 * the model they return. The layouts are documented in internal/kaops.
 */
internal object KcOps {
    const val OPEN = 10
    const val UPDATE = 11
    const val CLOSE = 12
    const val CLASS = 13
    const val PACKAGE = 14

    class Config(
        val jdkHome: String?,
        val moduleName: String,
        val jobs: Int,
        val classPath: List<String>,
        /** The entries of [classPath] whose internal declarations the module sees. */
        val friendPaths: List<String> = emptyList(),
    )

    fun open(cfg: Config): Long {
        val w = Wire.Writer()
            .writeString(cfg.jdkHome ?: "")
            .writeBool(cfg.jdkHome == null)
            .writeString(cfg.moduleName)
            .writeUint(cfg.jobs.toLong())
            .writeUint(cfg.classPath.size.toLong())
        cfg.classPath.forEach { w.writeString(it) }
        w.writeUint(cfg.friendPaths.size.toLong())
        cfg.friendPaths.forEach { w.writeString(it) }
        return Wire.Reader(KcNative.call(OPEN, w.message())).readInt()
    }

    /**
     * Checks [paths] in session [session] and returns their declarations,
     * or, if the check is one of [kept], only that (KcModel.same).
     * [bases] are the files of models read before, by their checks and
     * paths: a file the new model shares with one of them is that one,
     * not sent again. The model's handle, which [cls] reads library
     * classes through, is the caller's to [close].
     */
    fun update(
        session: Long,
        paths: List<String>,
        kept: Collection<Long> = emptyList(),
        bases: Map<Long, Map<String, KcFile>> = emptyMap(),
    ): KcModel {
        val w = Wire.Writer().writeInt(session).writeUint(paths.size.toLong())
        paths.forEach { w.writeString(it) }
        w.writeUint(kept.size.toLong())
        kept.forEach { w.writeUint(it) }
        w.writeUint(bases.size.toLong())
        bases.keys.forEach { w.writeUint(it) }
        val t = KcTimer()
        val r = Wire.Reader(KcNative.call(UPDATE, w.message()))
        t.mark("native")
        val model = r.readInt()
        try {
            val check = r.readUint()
            if (r.readBool()) {
                t.print("update same")
                return KcModel(model, emptyList(), emptyList(), check, same = true)
            }
            var reused = 0
            val files = List(r.readUint().toInt()) {
                val from = r.readUint()
                val path = r.readString()
                if (from == 0L) {
                    KcFile(path, r.readString(), r.annotations(), r.decls(), r.lookups(), r.readUint().toInt(), r.readUint(), r.readUint())
                } else {
                    reused++
                    bases[from]?.get(path) ?: error("kc: no file $path of check $from")
                }
            }
            val diagnostics = List(r.readUint().toInt()) {
                KcDiagnostic(r.readString(), r.readUint().toInt(), r.readUint().toInt(), r.readString(), r.readString())
            }
            t.mark("decode")
            t.print("update reused=$reused")
            return KcModel(model, files, diagnostics, check)
        } catch (e: Throwable) {
            close(model)
            throw e
        }
    }

    /** The class-path class [classId] (a/b/Outer.Inner) as model [model] has it, or null. */
    fun cls(model: Long, classId: String): KcDecl? {
        val r = Wire.Reader(KcTimer.timed(KcTimer.classCalls) { KcNative.call(CLASS, Wire.Writer().writeInt(model).writeString(classId).message()) })
        return r.decls().singleOrNull()
    }

    /** The class-path package [fqName] (dotted) as model [model] has it, or null if there is none. */
    fun pkg(model: Long, fqName: String): KcPackage? {
        val r = Wire.Reader(KcTimer.timed(KcTimer.packageCalls) { KcNative.call(PACKAGE, Wire.Writer().writeInt(model).writeString(fqName).message()) })
        val exists = r.readBool()
        val classes = List(r.readUint().toInt()) { r.readString() }
        val decls = r.decls()
        return if (exists) KcPackage(fqName, classes, decls) else null
    }

    fun close(handle: Long) {
        KcNative.call(CLOSE, Wire.Writer().writeInt(handle).message())
    }

    private fun Wire.Reader.annotations(): List<KcAnnotation> = List(readUint().toInt()) { annotation() }

    private fun Wire.Reader.lookups(): List<KcLookup> =
        List(readUint().toInt()) { KcLookup(readString(), readUint().toInt(), readString()) }

    private fun Wire.Reader.annotation(): KcAnnotation {
        val classId = readString()
        val node = readUint().toInt()
        val target = readString()
        val implied = readUint() == 1L
        val args = List(readUint().toInt()) { readString() to value()!! }
        return KcAnnotation(classId, node, target, implied, args, List(readUint().toInt()) { type() })
    }

    private fun Wire.Reader.value(): KcValue? {
        val kind = readUint().toInt()
        if (kind == 0) return null
        val node = readUint().toInt()
        return when (kind) {
            KcValue.CONST -> when (val c = readUint().toInt()) {
                KcValue.Const.NULL -> KcValue.Const(node, c, 0, 0.0, "")
                KcValue.Const.FLOAT, KcValue.Const.DOUBLE ->
                    KcValue.Const(node, c, 0, java.lang.Double.longBitsToDouble(readUint()), "")
                KcValue.Const.STRING -> KcValue.Const(node, c, 0, 0.0, readString())
                else -> KcValue.Const(node, c, readInt(), 0.0, "")
            }
            KcValue.ENUM -> KcValue.Enum(node, readString(), readString())
            KcValue.CLASS -> KcValue.ClassLit(node, type()!!)
            KcValue.ANNOTATION -> KcValue.Nested(node, annotation())
            KcValue.ARRAY -> KcValue.Array(node, List(readUint().toInt()) { value()!! })
            else -> KcValue.Unsupported(node)
        }
    }

    private fun Wire.Reader.decls(): List<KcDecl> =
        List(readUint().toInt()) {
            KcDecl(
                kind = readUint().toInt(),
                name = readString(),
                node = readUint().toInt(),
                classId = readString(),
                visibility = readUint().toInt(),
                modality = readUint().toInt(),
                flags = readUint().toLong(),
                variance = readUint().toInt(),
                type = type(),
                receiver = type(),
                supertypes = List(readUint().toInt()) { type()!! },
                bounds = List(readUint().toInt()) { type()!! },
                annotations = annotations(),
                typeParams = decls(),
                params = decls(),
                members = decls(),
                default = value(),
                locals = decls(),
                module = readString(),
                inheritors = List(readUint().toInt()) { readString() },
            )
        }

    private fun Wire.Reader.type(): KcType? {
        val kind = readUint().toInt()
        if (kind == 0) return null
        return KcType(
            kind = kind,
            classId = readString(),
            flags = readUint().toInt(),
            param = readUint().toInt(),
            name = readString(),
            contexts = readUint().toInt(),
            args = List(readUint().toInt()) { KcTypeArg(readUint().toInt(), type()) },
            abbrev = type(),
            annotations = annotations(),
        )
    }
}

/** The module's sources as one update saw them; Go's ka.Model. */
/**
 * The model of an update: its [handle], and its [check] (Go's Model.Check).
 * If [same], the update asked with a kept check and this is it again: the
 * model's files and diagnostics are left out, as they are that check's.
 */
internal class KcModel(
    val handle: Long,
    val files: List<KcFile>,
    val diagnostics: List<KcDiagnostic>,
    val check: Long = 0,
    val same: Boolean = false,
)

/** An error in the sources; Go's ka.Diagnostic. */
internal class KcDiagnostic(val path: String, val line: Int, val col: Int, val message: String, val feature: String) {
    override fun toString() = "$path:$line:$col: " + (if (feature.isEmpty()) "" else "unsupported: $feature: ") + message
}

/**
 * A class-path package; Go's ka.Package: its top-level classes' class IDs,
 * and its top-level functions, properties (whose classId is their JVM
 * facade's) and type aliases.
 */
internal class KcPackage(val fqName: String, val classes: List<String>, val decls: List<KcDecl>)

/** A source file's declarations; Go's ka.File. */
internal class KcFile(
    val path: String,
    val packageName: String,
    val annotations: List<KcAnnotation>,
    val decls: List<KcDecl>,
    /** The names the file looks up, for incremental processing; Go's ka.File.Lookups. */
    val lookups: List<KcLookup>,
    /** The number of kc's psi nodes of the file, and the hash of their kinds ([KcKinds.hash]). */
    val nodes: Int = -1,
    val kinds: Long = 0,
    /** The hash of the file's text as libkc read it (KcEnvironment's textHash), or 0. */
    val text: Long = 0,
) {
    /**
     * The IDs of the file's classes and type aliases, local ones too, in
     * preorder, aliases' marked: what a round indexes (KcRound.index).
     * A file the next update shares is the same KcFile, so each round
     * after the first doesn't parse them again.
     */
    val classIds: List<Pair<org.jetbrains.kotlin.name.ClassId, Boolean>> by lazy(LazyThreadSafetyMode.PUBLICATION) {
        val ids = ArrayList<Pair<org.jetbrains.kotlin.name.ClassId, Boolean>>()
        fun walk(ds: List<KcDecl>) {
            for (d in ds) {
                when (d.kind) {
                    KcDecl.CLASS, KcDecl.INTERFACE, KcDecl.OBJECT, KcDecl.ENUM_CLASS, KcDecl.ANNOTATION_CLASS ->
                        if (d.classId.isNotEmpty()) ids += org.jetbrains.kotlin.name.ClassId.fromString(d.classId) to false
                    KcDecl.TYPE_ALIAS ->
                        if (d.classId.isNotEmpty()) ids += org.jetbrains.kotlin.name.ClassId.fromString(d.classId) to true
                }
                walk(d.members)
                walk(d.locals)
                walk(d.params)
            }
        }
        walk(decls)
        ids
    }
}

/**
 * A name looked up in a package or a class, [scope] its qualified name;
 * Go's ka.Lookup. [kind] is [PACKAGE] or [CLASS].
 */
internal class KcLookup(val scope: String, val kind: Int, val name: String) {
    companion object {
        const val PACKAGE = 0
        const val CLASS = 1
    }
}

/**
 * A declaration; Go's ka.Decl. [node] is the declaration's psi node, a
 * preorder index into its file's tree, or 0 if it has none (a primary
 * constructor that isn't written).
 */
internal class KcDecl(
    val kind: Int,
    val name: String,
    val node: Int,
    val classId: String,
    val visibility: Int,
    val modality: Int,
    val flags: Long,
    val variance: Int,
    /** A callable's return type, a property's or parameter's type (a vararg's element type), a type alias's expansion. */
    val type: KcType?,
    val receiver: KcType?,
    val supertypes: List<KcType>,
    /** A type parameter's upper bounds, as written. */
    val bounds: List<KcType>,
    val annotations: List<KcAnnotation>,
    val typeParams: List<KcDecl>,
    val params: List<KcDecl>,
    val members: List<KcDecl>,
    /** An annotation class's constructor parameter's default value. */
    val default: KcValue? = null,
    /** The local declarations written directly in a function's, a constructor's or a property's accessors' bodies. */
    val locals: List<KcDecl> = emptyList(),
    /** A library Kotlin class's or top-level callable's module name; "" for "main". */
    val module: String = "",
    /** A library sealed class's direct subclasses' class IDs, from its metadata. */
    val inheritors: List<String> = emptyList(),
) {
    fun has(flag: Long) = flags and flag != 0L
    val isCompanion get() = has(COMPANION)
    val isData get() = has(DATA)

    companion object {
        // Go's ka.Kind values.
        const val CLASS = 1
        const val INTERFACE = 2
        const val OBJECT = 3
        const val ENUM_CLASS = 4
        const val ANNOTATION_CLASS = 5
        const val ENUM_ENTRY = 6
        const val FUNCTION = 7
        const val PROPERTY = 8
        const val TYPE_ALIAS = 9
        const val CONSTRUCTOR = 10
        const val VALUE_PARAMETER = 11
        const val TYPE_PARAMETER = 12
        const val JAVA_FIELD = 13
        const val DESTRUCTURING = 14

        // Go's ka.Visibility values.
        const val PUBLIC = 0
        const val PRIVATE = 1
        const val PROTECTED = 2
        const val INTERNAL = 3
        const val PACKAGE_PRIVATE = 4
        const val LOCAL = 5

        // Go's ka.Modality values.
        const val FINAL = 0
        const val OPEN = 1
        const val ABSTRACT = 2
        const val SEALED = 3

        // Go's ka.Flags bits.
        const val COMPANION = 1L shl 0
        const val DATA = 1L shl 1
        const val INNER = 1L shl 2
        const val VALUE = 1L shl 3
        const val FUN = 1L shl 4
        const val EXPECT = 1L shl 5
        const val ACTUAL = 1L shl 6
        const val INLINE = 1L shl 7
        const val SUSPEND = 1L shl 8
        const val OPERATOR = 1L shl 9
        const val INFIX = 1L shl 10
        const val TAILREC = 1L shl 11
        const val EXTERNAL = 1L shl 12
        const val CONST = 1L shl 13
        const val LATEINIT = 1L shl 14
        const val MUTABLE = 1L shl 15
        const val VARARG = 1L shl 16
        const val NOINLINE = 1L shl 17
        const val CROSSINLINE = 1L shl 18
        const val HAS_DEFAULT = 1L shl 19
        const val OVERRIDE = 1L shl 20
        const val REIFIED = 1L shl 21
        const val JAVA = 1L shl 22
        const val STATIC = 1L shl 23
        const val PRIMARY = 1L shl 24
        const val BACKING_FIELD = 1L shl 25
        /** A class of the module's Java sources (with [JAVA]). */
        const val JAVA_SOURCE = 1L shl 26
        /** A source enum entry with a body, whose [members] are the body's. */
        const val ENTRY_BODY = 1L shl 27

        // Go's ka.Variance values.
        const val INVARIANT = 0
        const val IN = 1
        const val OUT = 2
        const val STAR = 3
    }
}

/**
 * A type; Go's ka.Type. [param] is a type parameter type's TYPE_PARAMETER
 * node in the file of the declaration it is in, 0 if it isn't in sources.
 */
internal class KcType(
    val kind: Int,
    val classId: String,
    val flags: Int,
    val param: Int,
    val name: String,
    val contexts: Int,
    val args: List<KcTypeArg>,
    /** The typealias type this type was written as: a class type of the typealias's class ID. */
    val abbrev: KcType? = null,
    /** The type's annotations as written, @A T; [abbrev]'s too. */
    val annotations: List<KcAnnotation> = emptyList(),
) {
    val isNullable get() = flags and 1 != 0
    val isFlexible get() = flags and 2 != 0
    val isDefinitelyNonNull get() = flags and 4 != 0
    val isExtension get() = flags and 8 != 0
    /** An inferred type's error type, not an unresolved class's (Go's ka.Type.Inferred). */
    val isInferred get() = flags and 16 != 0

    companion object {
        // Go's ka.TypeKind values.
        const val CLASS = 1
        const val PARAMETER = 2
        const val ERROR = 3
    }
}

/** A type argument; [type] is null for a star projection. */
internal class KcTypeArg(val variance: Int, val type: KcType?)

/**
 * An annotation application; [node] is its ANNOTATION_ENTRY (a nested
 * annotation's CALL_EXPRESSION), [classId] is "" if kc didn't resolve it,
 * and [args] are its arguments as written, by parameter name.
 */
// target is the use-site target as written, or "" for none; implied is set
// when Kotlin's default rule chose it ("field") for one written without.
/** [typeArgs] are the type arguments written, @A<String>; null for a star projection. */
internal class KcAnnotation(val classId: String, val node: Int, val target: String, val implied: Boolean,
                            val args: List<Pair<String, KcValue>>, val typeArgs: List<KcType?> = emptyList())

/**
 * An annotation argument's value; Go's ka.Value. [node] is the expression
 * it is written as, 0 if it isn't (a vararg parameter's arguments).
 */
internal sealed class KcValue(val node: Int) {
    /** A constant of Go's ka.ConstKind [kind]: [bits] for integers, Boolean and Char, [float] for Float and Double. */
    class Const(node: Int, val kind: Int, val bits: Long, val float: Double, val str: String) : KcValue(node) {
        companion object {
            const val NULL = 0
            const val BOOLEAN = 1
            const val CHAR = 2
            const val BYTE = 3
            const val UBYTE = 4
            const val SHORT = 5
            const val USHORT = 6
            const val INT = 7
            const val UINT = 8
            const val LONG = 9
            const val ULONG = 10
            const val FLOAT = 11
            const val DOUBLE = 12
            const val STRING = 13
        }
    }

    class Enum(node: Int, val classId: String, val entry: String) : KcValue(node)
    class ClassLit(node: Int, val type: KcType) : KcValue(node)
    class Nested(node: Int, val annotation: KcAnnotation) : KcValue(node)
    class Array(node: Int, val elems: List<KcValue>) : KcValue(node)
    class Unsupported(node: Int) : KcValue(node)

    companion object {
        // Go's ka.ValueKind values.
        const val CONST = 1
        const val ENUM = 2
        const val CLASS = 3
        const val ANNOTATION = 4
        const val ARRAY = 5
        const val UNSUPPORTED = 6
    }
}

/**
 * KcTimer measures consecutive phases when the environment has KC_TIMING,
 * and writes them, in ms, to standard error on a line that starts with
 * "kc-jvm:" (bench/ksp).
 */
internal class KcTimer(start: Long = System.nanoTime()) {
    private var last = start
    private val parts = StringBuilder()

    fun mark(name: String) {
        if (!ON) return
        val now = System.nanoTime()
        parts.append(' ').append(name).append('=').append("%.1f".format((now - last) / 1e6))
        // Formatting isn't counted in the next part.
        last = System.nanoTime()
    }

    fun print(what: String) {
        if (ON) System.err.println("kc-jvm: $what$parts")
    }

    companion object {
        val ON = System.getenv("KC_TIMING") != null

        /** The count and ns of the library lookups (cls, pkg) since the last [printCalls]. */
        val classCalls = LongArray(2)
        val packageCalls = LongArray(2)

        inline fun <T> timed(calls: LongArray, f: () -> T): T {
            if (!ON) return f()
            val start = System.nanoTime()
            try {
                return f()
            } finally {
                calls[0]++
                calls[1] += System.nanoTime() - start
            }
        }

        fun printCalls() {
            if (!ON) return
            System.err.println("kc-jvm: lookups class=${classCalls[0]}/%.1f package=${packageCalls[0]}/%.1f".format(classCalls[1] / 1e6, packageCalls[1] / 1e6))
            classCalls.fill(0)
            packageCalls.fill(0)
        }
    }
}
