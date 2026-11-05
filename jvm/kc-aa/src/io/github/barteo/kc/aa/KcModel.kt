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
    fun update(session: Long, paths: List<String>): List<KcFile> {
        val w = Wire.Writer().writeInt(session).writeUint(paths.size.toLong())
        paths.forEach { w.writeString(it) }
        val r = Wire.Reader(KcNative.call(UPDATE, w.message()))
        val model = r.readInt()
        try {
            return List(r.readUint().toInt()) {
                KcFile(r.readString(), r.readString(), r.annotations(), r.decls())
            }
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
                flags = readUint().toInt(),
                annotations = annotations(),
                typeParams = decls(),
                params = decls(),
                members = decls(),
            )
        }
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
    val flags: Int,
    val annotations: List<KcAnnotation>,
    val typeParams: List<KcDecl>,
    val params: List<KcDecl>,
    val members: List<KcDecl>,
) {
    val isCompanion get() = flags and 1 != 0
    val isData get() = flags and 2 != 0

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
    }
}

/** An annotation application; [node] is its ANNOTATION_ENTRY. */
internal class KcAnnotation(val classId: String, val node: Int, val target: String)
