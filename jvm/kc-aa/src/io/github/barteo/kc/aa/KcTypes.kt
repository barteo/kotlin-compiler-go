@file:Suppress("DEPRECATION", "DEPRECATION_ERROR", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.aa

import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.base.KaContextReceiver
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.symbols.KaClassLikeSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbolLocation
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbolOrigin
import org.jetbrains.kotlin.analysis.api.symbols.KaTypeAliasSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaTypeParameterSymbol
import org.jetbrains.kotlin.analysis.api.symbols.pointers.KaSymbolPointer
import org.jetbrains.kotlin.analysis.api.symbols.KaClassifierSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaResolvedClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaUnresolvedClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaStarTypeProjection
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.components.KaBuiltinTypes
import org.jetbrains.kotlin.analysis.api.components.KaClassTypeBuilder
import org.jetbrains.kotlin.analysis.api.components.KaTypeParameterTypeBuilder
import org.jetbrains.kotlin.analysis.api.types.KaTypeArgumentWithVariance
import org.jetbrains.kotlin.analysis.api.types.KaTypeNullability
import org.jetbrains.kotlin.analysis.api.types.KaTypeProjection
import org.jetbrains.kotlin.analysis.api.types.KaUsualClassType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.Variance

/**
 * Where a type from the Go model is read: its round, the file whose psi
 * nodes its type parameter types name, and, for a type parameter type
 * without a node (a library declaration's), the type parameters in scope.
 */
internal class KcTypeContext(
    val round: KcRound,
    private val psi: (Int) -> PsiElement?,
    private val outer: KcTypeContext? = null,
    /** The KaAnnotation of a type's annotation; null where types have none. */
    private val annotate: ((KcAnnotation) -> KaAnnotation)? = null,
) {
    /** The type parameters the declaration this context is for declares. */
    var typeParams: List<KaTypeParameterSymbol> = emptyList()

    /** A context for a declaration nested in this one's. */
    fun nested() = KcTypeContext(round, psi, this, annotate)

    private fun byName(name: Name): KaTypeParameterSymbol? =
        typeParams.firstOrNull { it.name == name } ?: outer?.byName(name)

    /** The KaType of [t]. */
    fun type(t: KcType): KaType = when (t.kind) {
        KcType.CLASS -> if (t.isFlexible) {
            KcFlexibleType(round, classType(t, false), classType(t, true, readOnly[t.classId], t.classId == "kotlin/Array"))
        } else {
            classType(t, t.isNullable)
        }
        KcType.PARAMETER -> {
            val p = KcTypeParameterType(round, Name.identifier(t.name), parameter(t), t.isNullable, annotations(t))
            when {
                t.isDefinitelyNonNull -> KcDefinitelyNotNullType(round, p)
                t.isFlexible -> KcFlexibleType(round, p.withNullability(false), p.withNullability(true))
                else -> p
            }
        }
        else -> if (t.isFlexible) {
            KcFlexibleType(round, errorType(round, t.name, false), errorType(round, t.name, true))
        } else {
            errorType(round, t.name, t.isNullable)
        }
    }

    private fun annotations(t: KcType): List<KaAnnotation> =
        if (t.annotations.isEmpty() || annotate == null) emptyList() else t.annotations.map(annotate)

    /** [t] as a class type; [as] replaces its class, for a Java collection's read-only upper bound. */
    private fun classType(t: KcType, nullable: Boolean, `as`: String? = null, out: Boolean = false): KcBaseClassType {
        // A local class's type names its declaration.
        val localPsi = if (t.param != 0 && `as` == null) psi(t.param) else null
        val id = if (localPsi == null) ClassId.fromString(`as` ?: t.classId) else t.classId.lastIndexOf('/').let { i ->
            ClassId(FqName(if (i < 0) "" else t.classId.substring(0, i).replace('/', '.')), FqName(t.classId.substring(i + 1)), true)
        }
        // A Java array's upper bound is Array<out T>?.
        val args = if (out) args(t).map { if (it is KcTypeArgument && it.variance == Variance.INVARIANT) KcTypeArgument(it.type, Variance.OUT_VARIANCE, round.token) else it } else args(t)
        val annots = annotations(t)
        val abbrev = t.abbrev?.takeIf { it.kind == KcType.CLASS }?.let {
            KcClassType(round, ClassId.fromString(it.classId), args(it), nullable, isAlias = true, annots = annots)
        }
        return if (isFunctionClass(t.classId)) {
            KcFunctionType(round, id, args(t), nullable, t.isExtension, t.contexts, abbrev, annots)
        } else {
            KcClassType(round, id, args, nullable, abbrev = abbrev, annots = annots, localPsi = localPsi)
        }
    }

    private fun args(t: KcType): List<KaTypeProjection> = t.args.map { a ->
        val at = a.type
        if (a.variance == KcDecl.STAR || at == null) KcStarProjection(round.token)
        else KcTypeArgument(type(at), variance(a.variance), round.token)
    }

    private fun parameter(t: KcType): KaTypeParameterSymbol =
        psi(t.param)?.let { round.byPsi[it] as? KaTypeParameterSymbol }
            ?: byName(Name.identifier(t.name))
            ?: KcLibraryTypeParameterSymbol(round, Name.identifier(t.name))

    companion object {
        fun variance(v: Int) = when (v) {
            KcDecl.IN -> Variance.IN_VARIANCE
            KcDecl.OUT -> Variance.OUT_VARIANCE
            else -> Variance.INVARIANT
        }

        /**
         * The read-only collection classes of the mutable ones: a Java collection's flexible
         * type is `(MutableList<T>..List<T>?)`, as in FIR.
         */
        private val readOnly = listOf(
            "Iterable", "Collection", "List", "Set", "Map", "Iterator", "ListIterator",
        ).associate { "kotlin/collections/Mutable$it" to "kotlin/collections/$it" } +
            ("kotlin/collections/MutableMap.MutableEntry" to "kotlin/collections/Map.Entry")

        private val functionClass = Regex("""kotlin/((coroutines/Suspend)?|reflect/K(Suspend)?)Function\d+""")
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

/**
 * A class type, with its arguments. [abbrev] is the typealias type it was
 * written as. A type with [isAlias] set is such a typealias type, whose
 * [classId] is the typealias's and whose symbol is the typealias; the type
 * relations expand it first (see [KcRound.expand]).
 */
internal class KcClassType(
    private val round: KcRound,
    override val classId: ClassId,
    override val typeArguments: List<KaTypeProjection> = emptyList(),
    override val marked: Boolean = false,
    val abbrev: KcClassType? = null,
    val isAlias: Boolean = false,
    private val annots: List<KaAnnotation> = emptyList(),
    /** The local class this is the type of, whose [classId] is local and names no class. */
    private val local: KaClassLikeSymbol? = null,
    /** The declaration of the local class this is the type of, if [local] isn't known yet. */
    private val localPsi: PsiElement? = null,
) : StubKaUsualClassType(), KcBaseClassType {
    override val token get() = round.token
    private val localSymbol: KaClassLikeSymbol? get() = local ?: localPsi?.let { round.byPsi[it] as? KaClassLikeSymbol }
    override val symbol: KaClassLikeSymbol
        get() = localSymbol ?: if (isAlias) round.typeAlias(classId) ?: round.classSymbol(classId) else round.classSymbol(classId)
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation: KaUsualClassType? get() = abbrev
    override val annotations: KaAnnotationList get() = KcAnnotationList(annots, round.token)
    override val qualifiers: List<KaResolvedClassTypeQualifier> get() = qualifiers(round, classId, typeArguments)
    override fun withNullability(nullable: Boolean): KcClassType =
        if (nullable == marked) this
        else KcClassType(round, classId, typeArguments, nullable, abbrev?.withNullability(nullable), isAlias, annots, local, localPsi)
    // The abbreviation isn't part of the type's identity, as in FIR.
    override fun equals(other: Any?) = other is KcClassType && other.classId == classId && other.localSymbol == localSymbol &&
        other.marked == marked && other.isAlias == isAlias && other.typeArguments == typeArguments
    override fun hashCode() = (classId.hashCode() * 31 + typeArguments.hashCode()) * 2 + if (marked) 1 else 0
    override fun toString() = text(marked, classId.asFqNameString() +
        if (typeArguments.isEmpty()) "" else typeArguments.joinToString(", ", "<", ">"))
}

/**
 * A function type, kotlin/FunctionN or kotlin/coroutines/SuspendFunctionN,
 * or a reflection one, kotlin/reflect/KFunctionN or KSuspendFunctionN,
 * whose arguments are its context parameters' types, its receiver's, its
 * parameters' and its return type.
 */
internal class KcFunctionType(
    private val round: KcRound,
    override val classId: ClassId,
    override val typeArguments: List<KaTypeProjection>,
    override val marked: Boolean,
    override val hasReceiver: Boolean,
    internal val contexts: Int,
    /** The typealias type the function type was written as. */
    val abbrev: KcClassType? = null,
    private val annots: List<KaAnnotation> = emptyList(),
) : StubKaFunctionType(), KcBaseClassType {
    override val token get() = round.token
    override val symbol: KaClassLikeSymbol get() = round.classSymbol(classId)
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation: KaUsualClassType? get() = abbrev
    override val annotations: KaAnnotationList get() = KcAnnotationList(annots, round.token)
    override val qualifiers: List<KaResolvedClassTypeQualifier> get() = qualifiers(round, classId, typeArguments)
    private fun arg(i: Int) = typeArguments[i].type ?: KcClassType(round, ClassId.fromString("kotlin/Any"), marked = true)
    override val receiverType get() = if (hasReceiver) arg(contexts) else null
    private val firstParam get() = contexts + if (hasReceiver) 1 else 0
    override val parameterTypes get() = (firstParam until typeArguments.size - 1).map(::arg)
    override val arity get() = typeArguments.size - 1 - firstParam
    override val returnType get() = arg(typeArguments.size - 1)
    override val isSuspend get() = "Suspend" in classId.relativeClassName.asString()
    override val isReflectType get() = classId.packageFqName.asString() == "kotlin.reflect"
    override val hasContextReceivers get() = contexts > 0
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override fun withNullability(nullable: Boolean) =
        if (nullable == marked) this
        else KcFunctionType(round, classId, typeArguments, nullable, hasReceiver, contexts, abbrev?.withNullability(nullable), annots)
    override fun equals(other: Any?) = other is KcFunctionType && other.classId == classId &&
        other.marked == marked && other.hasReceiver == hasReceiver && other.typeArguments == typeArguments
    override fun hashCode() = (classId.hashCode() * 31 + typeArguments.hashCode()) * 2 + if (marked) 1 else 0
    override fun toString() = text(marked, classId.asFqNameString() + typeArguments.joinToString(", ", "<", ">"))
}

internal class KcTypeParameterType(
    private val round: KcRound,
    override val name: Name,
    override val symbol: KaTypeParameterSymbol,
    override val marked: Boolean,
    private val annots: List<KaAnnotation> = emptyList(),
) : StubKaTypeParameterType(), KcTypeBase {
    override val token get() = round.token
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(annots, round.token)
    override fun withNullability(nullable: Boolean) =
        if (nullable == marked) this else KcTypeParameterType(round, name, symbol, nullable, annots)
    override fun equals(other: Any?) = other is KcTypeParameterType && other.symbol == symbol && other.marked == marked
    override fun hashCode() = symbol.hashCode() * 2 + if (marked) 1 else 0
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
    // Its lower bound's, as FIR's flexible type's attributes are.
    override val annotations: KaAnnotationList get() = lowerBound.annotations
    override fun withNullability(nullable: Boolean) = (if (nullable) upperBound else lowerBound)
    override fun equals(other: Any?) = other is KcFlexibleType && other.lowerBound == lowerBound && other.upperBound == upperBound
    override fun hashCode() = lowerBound.hashCode() * 31 + upperBound.hashCode()
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
 * A type kc couldn't resolve: an unresolved class named [name], or, where
 * there is no name (a parameter written without a type), a plain error
 * type without text, as FIR gives.
 */
internal fun errorType(round: KcRound, name: String?, marked: Boolean = false): KaType =
    if (name.isNullOrEmpty()) KcMissingType(round, marked) else KcErrorType(round, name, marked)

/** An error type that names no class; see [errorType]. */
internal class KcMissingType(
    private val round: KcRound,
    override val marked: Boolean = false,
) : StubKaErrorType(), KcTypeBase {
    override val token get() = round.token
    override val nullability get() = KaTypeNullability.create(marked)
    override val abbreviation get() = null
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), round.token)
    override val errorMessage: String get() = "no type"
    override val presentableText: String? get() = null
    override fun withNullability(nullable: Boolean) = if (nullable == marked) this else KcMissingType(round, nullable)
    override fun toString() = "ERROR"
}

/**
 * The qualifiers of a class type: one per class from the outermost to
 * [classId]'s. kc keeps an inner class's arguments first and then its
 * outer classes', Outer<A>.Inner<B> as [B, A], so each inner class takes
 * as many as it has type parameters and the first non-inner one the rest.
 */
internal fun qualifiers(round: KcRound, classId: ClassId, args: List<KaTypeProjection>): List<KaResolvedClassTypeQualifier> {
    val out = ArrayList<KaResolvedClassTypeQualifier>()
    var id: ClassId? = classId
    var rest = args
    while (id != null) {
        val c = if (id.outerClassId != null) round.classSymbol(id) else null
        val own = if (c != null && c.isInner) rest.take(c.typeParameters.size) else rest
        out += KcQualifier(round, id, own)
        rest = rest.drop(own.size)
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

/** The builder `buildClassType` runs its block on. */
@Suppress("DEPRECATION_ERROR")
internal class KcClassTypeBuilder(override val token: KaLifetimeToken) : KaClassTypeBuilder {
    override var isMarkedNullable = false
    override var nullability: KaTypeNullability
        get() = KaTypeNullability.create(isMarkedNullable)
        set(value) {
            isMarkedNullable = value == KaTypeNullability.NULLABLE
        }
    override val arguments = ArrayList<KaTypeProjection>()
    override fun argument(argument: KaTypeProjection) {
        // KSP's own projections compare by identity; kc's are structural.
        arguments += when (argument) {
            is KaStarTypeProjection -> KcStarProjection(token)
            is KaTypeArgumentWithVariance -> KcTypeArgument(argument.type, argument.variance, token)
            else -> argument
        }
    }
    override fun argument(type: KaType, variance: Variance) {
        arguments += KcTypeArgument(type, variance, token)
    }

    fun build(round: KcRound, id: ClassId): KaType = KcClassType(round, id, arguments.toList(), isMarkedNullable)

    /** The type of local class [c]: its class ID is local, as in FIR. */
    fun build(round: KcRound, c: KaClassLikeSymbol, pkg: FqName): KaType =
        KcClassType(round, ClassId(pkg, FqName(c.name?.asString() ?: "<anonymous>"), true),
            arguments.toList(), isMarkedNullable, local = c)

    /** The type of typealias [alias] with the builder's arguments: its expansion, abbreviated by it. */
    fun build(round: KcRound, alias: KaTypeAliasSymbol): KaType =
        round.expand(KcClassType(round, alias.classId!!, arguments.toList(), isMarkedNullable, isAlias = true))
}

internal class KcTypeParameterTypeBuilder(override val token: KaLifetimeToken) : KaTypeParameterTypeBuilder {
    override var isMarkedNullable = false
    override var nullability: KaTypeNullability
        get() = KaTypeNullability.create(isMarkedNullable)
        set(value) {
            isMarkedNullable = value == KaTypeNullability.NULLABLE
        }
}
