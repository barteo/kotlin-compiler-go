package io.github.barteo.kc.aa

import org.jetbrains.kotlin.analysis.api.symbols.KaConstructorSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.analysis.api.types.KaDefinitelyNotNullType
import org.jetbrains.kotlin.analysis.api.types.KaErrorType
import org.jetbrains.kotlin.analysis.api.types.KaFlexibleType
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.types.KaTypeArgumentWithVariance
import org.jetbrains.kotlin.analysis.api.types.KaTypeParameterType
import org.jetbrains.kotlin.builtins.jvm.JavaToKotlinClassMap
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.types.Variance

/**
 * The JVM erasure of KaTypes: [descriptor] is what FIR's type mapper gives
 * in TypeMappingMode.DEFAULT, the descriptor of a value parameter's or a
 * return type's erasure (Unit stays kotlin/Unit; callers map it to V).
 */
internal class KcJvmTypes(private val round: KcRound) {
    fun descriptor(type: KaType): String = map(type, Position.TOP, 0)

    private enum class Position { TOP, ARRAY_ELEMENT }

    private fun map(t0: KaType, pos: Position, depth: Int): String {
        if (depth > 32) return OBJECT
        val t = round.expand(t0)
        return when (t) {
            is KaFlexibleType -> map(t.lowerBound, pos, depth + 1)
            is KaDefinitelyNotNullType -> map(t.original, pos, depth + 1)
            is KaErrorType -> "Lerror/NonExistentClass;"
            is KaTypeParameterType -> {
                val bound = t.symbol.upperBounds.firstOrNull() ?: return OBJECT
                // A type parameter's erasure is its bound's, boxed.
                map(bound.withNullability(true), pos, depth + 1)
            }
            is KaClassType -> classType(t, pos, depth)
            else -> OBJECT
        }
    }

    private fun KaType.withNullability(nullable: Boolean): KaType = (this as? KcTypeBase)?.withNullability(nullable) ?: this

    private fun classType(t: KaClassType, pos: Position, depth: Int): String {
        val id = t.classId
        val nullable = (t as? KcTypeBase)?.marked == true
        primitives[id]?.let { (prim, box) ->
            return if (nullable || pos == Position.ARRAY_ELEMENT) "L$box;" else prim
        }
        primitiveArrays[id]?.let { return it }
        when (id) {
            ARRAY -> {
                val arg = t.typeArguments.firstOrNull()
                val elem = arg?.type
                return "[" + if (elem == null || arg is KaTypeArgumentWithVariance && arg.variance == Variance.IN_VARIANCE) OBJECT
                else map(elem, Position.ARRAY_ELEMENT, depth + 1)
            }
            NOTHING -> return "Ljava/lang/Void;"
        }
        // A value class is its underlying type, unless boxing is needed.
        // (A class kc doesn't know is a KcLibraryClassSymbol, with no flags.)
        val cls = round.classSymbol(id) as? KcClassSymbol
        if (cls != null && cls.isInline && pos == Position.TOP) {
            val underlying = cls.members.filterIsInstance<KaConstructorSymbol>()
                .firstOrNull { it.isPrimary }?.valueParameters?.singleOrNull()?.returnType
            if (underlying != null) {
                val inner = round.expand(underlying)
                val boxed = nullable && ((inner as? KcTypeBase)?.marked == true || inner is KaTypeParameterType ||
                    inner is KaClassType && inner.classId in primitives)
                if (!boxed) return map(if (nullable) underlying.withNullability(true) else underlying, pos, depth + 1)
            }
        }
        return "L" + internalName(javaClass(id)) + ";"
    }

    /** The Java class a Kotlin class is on the JVM. */
    private fun javaClass(id: ClassId): ClassId {
        val rel = id.relativeClassName.asString()
        if (id.packageFqName.asString() == "kotlin.coroutines" && rel.startsWith("SuspendFunction")) {
            rel.removePrefix("SuspendFunction").toIntOrNull()?.let {
                return ClassId.fromString("kotlin/jvm/functions/Function${it + 1}")
            }
        }
        return JavaToKotlinClassMap.mapKotlinToJava(id.asSingleFqName().toUnsafe()) ?: id
    }

    private fun internalName(id: ClassId): String {
        val pkg = id.packageFqName.asString().replace('.', '/')
        val rel = id.relativeClassName.asString().replace('.', '$')
        return if (pkg.isEmpty()) rel else "$pkg/$rel"
    }

    companion object {
        private const val OBJECT = "Ljava/lang/Object;"
        private val ARRAY = ClassId.fromString("kotlin/Array")
        private val NOTHING = ClassId.fromString("kotlin/Nothing")

        private val primitives: Map<ClassId, Pair<String, String>> = mapOf(
            "Boolean" to ("Z" to "java/lang/Boolean"),
            "Char" to ("C" to "java/lang/Character"),
            "Byte" to ("B" to "java/lang/Byte"),
            "Short" to ("S" to "java/lang/Short"),
            "Int" to ("I" to "java/lang/Integer"),
            "Float" to ("F" to "java/lang/Float"),
            "Long" to ("J" to "java/lang/Long"),
            "Double" to ("D" to "java/lang/Double"),
        ).mapKeys { ClassId.fromString("kotlin/${it.key}") }

        private val primitiveArrays: Map<ClassId, String> = mapOf(
            "BooleanArray" to "[Z", "CharArray" to "[C", "ByteArray" to "[B", "ShortArray" to "[S",
            "IntArray" to "[I", "FloatArray" to "[F", "LongArray" to "[J", "DoubleArray" to "[D",
        ).mapKeys { ClassId.fromString("kotlin/${it.key}") }
    }
}
