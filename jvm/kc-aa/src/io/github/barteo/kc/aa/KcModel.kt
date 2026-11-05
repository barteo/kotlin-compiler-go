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

    class Config(
        val jdkHome: String?,
        val moduleName: String,
        val jobs: Int,
        val classPath: List<String>,
    )

    fun open(cfg: Config): Long {
        val w = Wire.Writer()
            .writeString(cfg.jdkHome ?: "")
            .writeBool(cfg.jdkHome == null)
            .writeString(cfg.moduleName)
            .writeUint(cfg.jobs.toLong())
            .writeUint(cfg.classPath.size.toLong())
        cfg.classPath.forEach { w.writeString(it) }
        return Wire.Reader(KcNative.call(OPEN, w.message())).readInt()
    }

    /** Checks [paths] in session [session] and returns their declarations. */
    fun update(session: Long, paths: List<String>): KcModel {
        val w = Wire.Writer().writeInt(session).writeUint(paths.size.toLong())
        paths.forEach { w.writeString(it) }
        val r = Wire.Reader(KcNative.call(UPDATE, w.message()))
        val model = r.readInt()
        try {
            val files = List(r.readUint().toInt()) {
                KcFile(r.readString(), r.readString(), r.annotations(), r.decls())
            }
            val diagnostics = List(r.readUint().toInt()) {
                KcDiagnostic(r.readString(), r.readUint().toInt(), r.readUint().toInt(), r.readString(), r.readString())
            }
            return KcModel(files, diagnostics)
        } finally {
            close(model)
        }
    }

    fun close(handle: Long) {
        KcNative.call(CLOSE, Wire.Writer().writeInt(handle).message())
    }

    private fun Wire.Reader.annotations(): List<KcAnnotation> =
        List(readUint().toInt()) { KcAnnotation(readString(), readUint().toInt(), readString()) }

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
        )
    }
}

/** The module's sources as one update saw them; Go's ka.Model. */
internal class KcModel(val files: List<KcFile>, val diagnostics: List<KcDiagnostic>)

/** An error in the sources; Go's ka.Diagnostic. */
internal class KcDiagnostic(val path: String, val line: Int, val col: Int, val message: String, val feature: String) {
    override fun toString() = "$path:$line:$col: " + (if (feature.isEmpty()) "" else "unsupported: $feature: ") + message
}

/** A source file's declarations; Go's ka.File. */
internal class KcFile(
    val path: String,
    val packageName: String,
    val annotations: List<KcAnnotation>,
    val decls: List<KcDecl>,
)

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

        // Go's ka.Visibility values.
        const val PUBLIC = 0
        const val PRIVATE = 1
        const val PROTECTED = 2
        const val INTERNAL = 3

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
) {
    val isNullable get() = flags and 1 != 0
    val isFlexible get() = flags and 2 != 0
    val isDefinitelyNonNull get() = flags and 4 != 0
    val isExtension get() = flags and 8 != 0

    companion object {
        // Go's ka.TypeKind values.
        const val CLASS = 1
        const val PARAMETER = 2
        const val ERROR = 3
    }
}

/** A type argument; [type] is null for a star projection. */
internal class KcTypeArg(val variance: Int, val type: KcType?)

/** An annotation application; [node] is its ANNOTATION_ENTRY, and [classId] is "" if kc didn't resolve it. */
internal class KcAnnotation(val classId: String, val node: Int, val target: String)
