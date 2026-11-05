@file:Suppress("DEPRECATION", "DEPRECATION_ERROR", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.aa

import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.base.KaContextReceiver
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.symbols.KaClassLikeSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbolLocation
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbolOrigin
import org.jetbrains.kotlin.analysis.api.symbols.KaTypeParameterSymbol
import org.jetbrains.kotlin.analysis.api.symbols.pointers.KaSymbolPointer
import org.jetbrains.kotlin.analysis.api.symbols.KaClassifierSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaResolvedClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaUnresolvedClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaStarTypeProjection
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.components.KaBuiltinTypes
import org.jetbrains.kotlin.analysis.api.types.KaTypeArgumentWithVariance
import org.jetbrains.kotlin.analysis.api.types.KaTypeNullability
import org.jetbrains.kotlin.analysis.api.types.KaTypeProjection
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.Variance

/**
 * Where a type from the Go model is read: its round, and the file whose psi
 * nodes its type parameter types name.
 */
internal class KcTypeContext(val round: KcRound, private val psi: (Int) -> PsiElement?) {
    /** The KaType of [t]. */
    fun type(t: KcType): KaType = when (t.kind) {
        KcType.CLASS -> if (t.isFlexible) {
            KcFlexibleType(round, classType(t, false), classType(t, true))
        } else {
            classType(t, t.isNullable)
        }
        KcType.PARAMETER -> {
            val p = KcTypeParameterType(round, Name.identifier(t.name), parameter(t), t.isNullable)
            when {
                t.isDefinitelyNonNull -> KcDefinitelyNotNullType(round, p)
                t.isFlexible -> KcFlexibleType(round, p.withNullability(false), p.withNullability(true))
                else -> p
            }
        }
        else -> KcErrorType(round, t.name, t.isNullable)
    }

    private fun classType(t: KcType, nullable: Boolean): KcBaseClassType {
        val id = ClassId.fromString(t.classId)
        val args = t.args.map { a ->
            val at = a.type
            if (a.variance == KcDecl.STAR || at == null) KcStarProjection(round.token)
            else KcTypeArgument(type(at), variance(a.variance), round.token)
        }
        return if (isFunctionClass(t.classId)) {
            KcFunctionType(round, id, args, nullable, t.isExtension, t.contexts)
        } else {
            KcClassType(round, id, args, nullable)
        }
    }

    private fun parameter(t: KcType): KaTypeParameterSymbol =
        psi(t.param)?.let { round.byPsi[it] as? KaTypeParameterSymbol }
            ?: KcLibraryTypeParameterSymbol(round, Name.identifier(t.name))

    companion object {
        fun variance(v: Int) = when (v) {
            KcDecl.IN -> Variance.IN_VARIANCE
            KcDecl.OUT -> Variance.OUT_VARIANCE
            else -> Variance.INVARIANT
        }

        private val functionClass = Regex("""kotlin/(coroutines/Suspend)?Function\d+""")
        fun isFunctionClass(id: String) = functionClass.matches(id)
    }
}

/** What kc's types have in common: whether they are marked nullable, and a copy that is or isn't. */
internal interface KcTypeBase : KaType {
    val marked: Boolean
    fun withNullability(nullable: Boolean): KaType
}

internal sealed interface KcBaseClassType : KcTypeBase {
    override fun withNullability(nullable: Boolean): KcBaseClassType
}

private fun KaType.text(marked: Boolean, s: String) = if (marked) "$s?" else s

/** A class type, with its arguments. */
internal class KcClassType(
    private val round: KcRound,
    override val classId: ClassId,
    override val typeArguments: List<KaTypeProjection> = emptyList(),
    override val marked: Boolean = false,
) : StubKaUsualClassType(), KcBaseClassType {
    override val token get() = round.token
    override val symbol: KaClassLikeSymbol get() = round.classSymbol(classId)
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override val qualifiers: List<KaResolvedClassTypeQualifier> get() = qualifiers(round, classId, typeArguments)
    override fun withNullability(nullable: Boolean) =
        if (nullable == marked) this else KcClassType(round, classId, typeArguments, nullable)
    override fun equals(other: Any?) = other is KcClassType && other.classId == classId &&
        other.marked == marked && other.typeArguments == typeArguments
    override fun hashCode() = classId.hashCode() * 31 + typeArguments.hashCode()
    override fun toString() = text(marked, classId.asFqNameString() +
        if (typeArguments.isEmpty()) "" else typeArguments.joinToString(", ", "<", ">"))
}

/**
 * A function type, kotlin/FunctionN or kotlin/coroutines/SuspendFunctionN,
 * whose arguments are its context parameters' types, its receiver's, its
 * parameters' and its return type.
 */
internal class KcFunctionType(
    private val round: KcRound,
    override val classId: ClassId,
    override val typeArguments: List<KaTypeProjection>,
    override val marked: Boolean,
    override val hasReceiver: Boolean,
    private val contexts: Int,
) : StubKaFunctionType(), KcBaseClassType {
    override val token get() = round.token
    override val symbol: KaClassLikeSymbol get() = round.classSymbol(classId)
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override val qualifiers: List<KaResolvedClassTypeQualifier> get() = qualifiers(round, classId, typeArguments)
    private fun arg(i: Int) = typeArguments[i].type ?: KcClassType(round, ClassId.fromString("kotlin/Any"), marked = true)
    override val receiverType get() = if (hasReceiver) arg(contexts) else null
    private val firstParam get() = contexts + if (hasReceiver) 1 else 0
    override val parameterTypes get() = (firstParam until typeArguments.size - 1).map(::arg)
    override val arity get() = typeArguments.size - 1 - firstParam
    override val returnType get() = arg(typeArguments.size - 1)
    override val isSuspend get() = classId.relativeClassName.asString().startsWith("Suspend")
    override val isReflectType get() = false
    override val hasContextReceivers get() = contexts > 0
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override fun withNullability(nullable: Boolean) =
        if (nullable == marked) this else KcFunctionType(round, classId, typeArguments, nullable, hasReceiver, contexts)
    override fun equals(other: Any?) = other is KcFunctionType && other.classId == classId &&
        other.marked == marked && other.hasReceiver == hasReceiver && other.typeArguments == typeArguments
    override fun hashCode() = classId.hashCode() * 31 + typeArguments.hashCode()
    override fun toString() = text(marked, classId.asFqNameString() + typeArguments.joinToString(", ", "<", ">"))
}

internal class KcTypeParameterType(
    private val round: KcRound,
    override val name: Name,
    override val symbol: KaTypeParameterSymbol,
    override val marked: Boolean,
) : StubKaTypeParameterType(), KcTypeBase {
    override val token get() = round.token
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override fun withNullability(nullable: Boolean) =
        if (nullable == marked) this else KcTypeParameterType(round, name, symbol, nullable)
    override fun equals(other: Any?) = other is KcTypeParameterType && other.symbol == symbol && other.marked == marked
    override fun hashCode() = symbol.hashCode()
    override fun toString() = text(marked, name.asString())
}

/** A platform type, T!, from Java. */
internal class KcFlexibleType(
    private val round: KcRound,
    override val lowerBound: KaType,
    override val upperBound: KaType,
) : StubKaFlexibleType(), KcTypeBase {
    override val token get() = round.token
    override val marked get() = false
    override val nullability get() = KaTypeNullability.UNKNOWN
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override fun withNullability(nullable: Boolean) = (if (nullable) upperBound else lowerBound)
    override fun equals(other: Any?) = other is KcFlexibleType && other.lowerBound == lowerBound && other.upperBound == upperBound
    override fun hashCode() = lowerBound.hashCode()
    override fun toString() = "$lowerBound!"
}

/** T & Any. */
internal class KcDefinitelyNotNullType(private val round: KcRound, override val original: KaType) :
    StubKaDefinitelyNotNullType(), KcTypeBase {
    override val token get() = round.token
    override val marked get() = false
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override fun withNullability(nullable: Boolean) = if (nullable) (original as KcTypeBase).withNullability(true) else this
    override fun equals(other: Any?) = other is KcDefinitelyNotNullType && other.original == original
    override fun hashCode() = original.hashCode() + 1
    override fun toString() = "$original & Any"
}

/** A type kc didn't resolve; [presentableText] is the type as written. */
internal class KcErrorType(
    private val round: KcRound,
    override val presentableText: String?,
    override val marked: Boolean = false,
) : StubKaClassErrorType(), KcTypeBase {
    override val token get() = round.token
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override val errorMessage: String get() = "unresolved: $presentableText"
    override val candidateSymbols: Collection<KaClassLikeSymbol> get() = emptyList()
    override val qualifiers: List<KaClassTypeQualifier>
        get() = listOf(KcUnresolvedQualifier(Name.guessByFirstCharacter(presentableText?.substringBefore('<')?.trim().orEmpty()), round.token))
    override fun withNullability(nullable: Boolean) = if (nullable == marked) this else KcErrorType(round, presentableText, nullable)
    override fun toString() = "ERROR($presentableText)"
}

/**
 * The qualifiers of a class type: one per class from the outermost to
 * [classId]'s, the last with all of [args]. kc keeps an inner class's
 * outer arguments with its own.
 */
internal fun qualifiers(round: KcRound, classId: ClassId, args: List<KaTypeProjection>): List<KaResolvedClassTypeQualifier> {
    val out = ArrayList<KaResolvedClassTypeQualifier>()
    var id: ClassId? = classId
    var own = args
    while (id != null) {
        out += KcQualifier(round, id, own)
        own = emptyList()
        id = id.outerClassId
    }
    return out.asReversed()
}

internal class KcQualifier(private val round: KcRound, private val classId: ClassId, override val typeArguments: List<KaTypeProjection>) :
    KaResolvedClassTypeQualifier {
    override val token get() = round.token
    override val name: Name get() = classId.shortClassName
    override val symbol: KaClassifierSymbol get() = round.classSymbol(classId)
}

internal class KcUnresolvedQualifier(override val name: Name, override val token: KaLifetimeToken) : KaUnresolvedClassTypeQualifier {
    override val typeArguments: List<KaTypeProjection> get() = emptyList()
}

internal class KcStarProjection(override val token: KaLifetimeToken) : KaStarTypeProjection {
    override fun equals(other: Any?) = other is KcStarProjection
    override fun hashCode() = 1
    override fun toString() = "*"
}

internal class KcTypeArgument(
    override val type: KaType,
    override val variance: Variance,
    override val token: KaLifetimeToken,
) : KaTypeArgumentWithVariance {
    override fun equals(other: Any?) = other is KcTypeArgument && other.type == type && other.variance == variance
    override fun hashCode() = type.hashCode() * 3 + variance.hashCode()
    override fun toString() = (if (variance == Variance.INVARIANT) "" else "${variance.label} ") + type
}

/** A type parameter outside the sources, known by name only until kc models library declarations. */
internal class KcLibraryTypeParameterSymbol(private val round: KcRound, override val name: Name) : StubKaTypeParameterSymbol() {
    override val token get() = round.token
    override val psi: PsiElement? get() = null
    override val origin get() = KaSymbolOrigin.LIBRARY
    override val location get() = KaSymbolLocation.LOCAL
    override val isReified get() = false
    override val variance get() = Variance.INVARIANT
    override val upperBounds: List<KaType> get() = emptyList()
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override fun createPointer(): KaSymbolPointer<KaTypeParameterSymbol> = throw UnsupportedOperationException("kc: pointer to library type parameter $name")
    override fun toString() = "KcLibraryTypeParameterSymbol($name)"
}

/** The built-in types, as the session's [KaBuiltinTypes]. */
internal class KcBuiltinTypes(private val round: KcRound) : KaBuiltinTypes() {
    override val token get() = round.token
    private fun t(id: String, nullable: Boolean = false) = KcClassType(round, ClassId.fromString(id), marked = nullable)
    override val int by lazy { t("kotlin/Int") }
    override val long by lazy { t("kotlin/Long") }
    override val short by lazy { t("kotlin/Short") }
    override val byte by lazy { t("kotlin/Byte") }
    override val float by lazy { t("kotlin/Float") }
    override val double by lazy { t("kotlin/Double") }
    override val boolean by lazy { t("kotlin/Boolean") }
    override val char by lazy { t("kotlin/Char") }
    override val string by lazy { t("kotlin/String") }
    override val unit by lazy { t("kotlin/Unit") }
    override val nothing by lazy { t("kotlin/Nothing") }
    override val any by lazy { t("kotlin/Any") }
    override val throwable by lazy { t("kotlin/Throwable") }
    override val nullableAny by lazy { t("kotlin/Any", true) }
    override val nullableNothing by lazy { t("kotlin/Nothing", true) }
    override val annotationType by lazy { t("kotlin/Annotation") }
}
