@file:Suppress("DEPRECATION", "DEPRECATION_ERROR", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.detekt

import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.base.KaContextReceiver
import org.jetbrains.kotlin.analysis.api.symbols.KaClassLikeSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaTypeParameterSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaResolvedClassTypeQualifier
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.types.KaTypeNullability
import org.jetbrains.kotlin.analysis.api.types.KaTypeProjection
import org.jetbrains.kotlin.analysis.api.types.KaUsualClassType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.KtTypeParameterListOwner
import org.jetbrains.kotlin.types.Variance

/** What kc's types have in common. */
internal interface KcTypeBase : KaType {
    /** Whether the type is marked nullable, T?. */
    val marked: Boolean

    /** Whether the type holds null. */
    val holdsNull: Boolean get() = marked

    fun withNullability(nullable: Boolean): KaType

    /**
     * The fully qualified names of the type's classifier and its
     * supertypes, as kc lists them: what subtyping checks read.
     */
    val classNames: Set<String>
}

internal object KcEmptyAnnotations : StubKaAnnotationList() {
    private val none = emptyList<KaAnnotation>()
    override val token get() = KcToken
    override val classIds: Collection<ClassId> get() = emptyList()
    override fun contains(classId: ClassId) = false
    override fun contains(element: KaAnnotation) = false
    override fun containsAll(elements: Collection<KaAnnotation>) = elements.isEmpty()
    override fun get(classId: ClassId): List<KaAnnotation> = emptyList()
    override fun get(index: Int): KaAnnotation = none[index]
    override fun indexOf(element: KaAnnotation) = -1
    override fun isEmpty() = true
    override fun iterator(): Iterator<KaAnnotation> = none.iterator()
    override fun lastIndexOf(element: KaAnnotation) = -1
    override fun listIterator(): ListIterator<KaAnnotation> = none.listIterator()
    override fun listIterator(index: Int): ListIterator<KaAnnotation> = none.listIterator(index)
    override val size: Int get() = 0
    override fun subList(fromIndex: Int, toIndex: Int): List<KaAnnotation> = none.subList(fromIndex, toIndex)
}

private fun nullabilityOf(marked: Boolean) = if (marked) KaTypeNullability.NULLABLE else KaTypeNullability.NON_NULLABLE

private fun text(marked: Boolean, s: String) = if (marked) "$s?" else s

/** A class type, with its arguments; [kc] is kc's type it was made from, if any. */
internal open class KcClassType(
    internal val session: KcSession,
    override val classId: ClassId,
    override val typeArguments: List<KaTypeProjection>,
    override val marked: Boolean,
    internal val kc: KcType?,
) : StubKaUsualClassType(), KcTypeBase {
    override val token get() = KcToken
    override val symbol: KaClassLikeSymbol get() = session.classSymbol(classId)
    override val nullability get() = nullabilityOf(marked)
    override val abbreviation: KaUsualClassType? get() = null
    override val annotations: KaAnnotationList get() = KcEmptyAnnotations
    override val qualifiers: List<KaResolvedClassTypeQualifier> get() = emptyList()
    override fun withNullability(nullable: Boolean): KaType =
        if (nullable == marked) this else KcClassType(session, classId, typeArguments, nullable, kc)

    override val classNames: Set<String> by lazy { classNames(session, classId, kc) }

    override fun equals(other: Any?) = other is KcClassType && other.classId == classId && other.marked == marked &&
        other.typeArguments == typeArguments
    override fun hashCode() = (classId.hashCode() * 31 + typeArguments.hashCode()) * 2 + if (marked) 1 else 0
    override fun toString() = text(marked, classId.asFqNameString() + if (typeArguments.isEmpty()) "" else typeArguments.joinToString(", ", "<", ">"))
}

/** The names subtyping reads of class [id]: its own and its supertypes', from kc's type or its source class. */
internal fun classNames(session: KcSession, id: ClassId, kc: KcType?): Set<String> {
    val out = LinkedHashSet<String>()
    out += id.asFqNameString()
    kc?.name?.let { out += it }
    if (kc != null && kc.supertypes.isNotEmpty()) {
        out += kc.supertypes
    } else {
        session.sourceClass(id)?.let { c -> session.sym(session.fact(c)?.symbol ?: 0)?.supertypes?.let { out += it } }
    }
    out += "kotlin.Any"
    return out
}

/** A function type, kotlin/FunctionN or kotlin/coroutines/SuspendFunctionN. */
internal class KcFunctionType(
    private val session: KcSession,
    override val classId: ClassId,
    override val typeArguments: List<KaTypeProjection>,
    override val marked: Boolean,
    private val kc: KcType?,
) : StubKaFunctionType(), KcTypeBase {
    override val token get() = KcToken
    override val symbol: KaClassLikeSymbol get() = session.classSymbol(classId)
    override val nullability get() = nullabilityOf(marked)
    override val abbreviation: KaUsualClassType? get() = null
    override val annotations: KaAnnotationList get() = KcEmptyAnnotations
    override val qualifiers: List<KaResolvedClassTypeQualifier> get() = emptyList()
    override val isSuspend: Boolean get() = classId.packageFqName.asString() == "kotlin.coroutines"
    override val isReflectType: Boolean get() = false
    override val arity: Int get() = typeArguments.size - 1
    override val hasContextReceivers: Boolean get() = false
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val hasReceiver: Boolean get() = false
    override val receiverType: KaType? get() = null
    override val parameterTypes: List<KaType> get() = typeArguments.dropLast(1).map { it.type ?: session.any }
    override val returnType: KaType get() = typeArguments.lastOrNull()?.type ?: session.unit
    override fun withNullability(nullable: Boolean): KaType =
        if (nullable == marked) this else KcFunctionType(session, classId, typeArguments, nullable, kc)
    override val classNames: Set<String> by lazy { classNames(session, classId, kc) }
    override fun equals(other: Any?) = other is KcFunctionType && other.classId == classId && other.marked == marked &&
        other.typeArguments == typeArguments
    override fun hashCode() = (classId.hashCode() * 31 + typeArguments.hashCode()) * 2 + if (marked) 1 else 0
    override fun toString() = text(marked, classId.asFqNameString() + typeArguments.joinToString(", ", "<", ">"))
}

/** A type parameter's type. */
internal class KcTypeParameterType(
    private val session: KcSession,
    private val kc: KcType,
    override val marked: Boolean,
) : StubKaTypeParameterType(), KcTypeBase {
    override val token get() = KcToken
    override val name: Name get() = Name.identifier(kc.name)
    override val holdsNull: Boolean get() = marked || kc.nullable
    override val symbol: KaTypeParameterSymbol
        get() {
            val owner = session.psi(kc.owner) as? KtTypeParameterListOwner
            val p = owner?.typeParameters?.firstOrNull { it.name == kc.name }
            return session.declarationSymbol(p) as? KaTypeParameterSymbol ?: KcLibraryTypeParameterSymbol(session, name)
        }
    override val nullability get() = nullabilityOf(marked)
    override val abbreviation: KaUsualClassType? get() = null
    override val annotations: KaAnnotationList get() = KcEmptyAnnotations
    override fun withNullability(nullable: Boolean): KaType = if (nullable == marked) this else KcTypeParameterType(session, kc, nullable)
    override val classNames: Set<String> get() = setOf("kotlin.Any")
    override fun equals(other: Any?) = other is KcTypeParameterType && other.kc.name == kc.name && other.kc.owner == kc.owner && other.marked == marked
    override fun hashCode() = kc.name.hashCode() * 2 + if (marked) 1 else 0
    override fun toString() = text(marked, kc.name)
}

/** A Java type of unknown nullability, T!: (T..T?). */
internal class KcFlexibleType(
    private val session: KcSession,
    override val lowerBound: KaType,
    override val upperBound: KaType,
) : StubKaFlexibleType(), KcTypeBase {
    override val token get() = KcToken
    override val marked: Boolean get() = false
    override val holdsNull: Boolean get() = true
    override val nullability get() = KaTypeNullability.UNKNOWN
    override val abbreviation: KaUsualClassType? get() = null
    override val annotations: KaAnnotationList get() = KcEmptyAnnotations
    override fun withNullability(nullable: Boolean): KaType = (if (nullable) upperBound else lowerBound)
    override val classNames: Set<String> get() = (lowerBound as? KcTypeBase)?.classNames ?: setOf("kotlin.Any")
    override fun equals(other: Any?) = other is KcFlexibleType && other.lowerBound == lowerBound && other.upperBound == upperBound
    override fun hashCode() = lowerBound.hashCode() * 31 + upperBound.hashCode()
    override fun toString() = "$lowerBound!"
}

/** A type kc couldn't resolve. */
internal class KcErrorType(private val session: KcSession, private val text: String, override val marked: Boolean = false) :
    StubKaClassErrorType(), KcTypeBase {
    override val token get() = KcToken
    override val errorMessage: String get() = "kc: unresolved type $text"
    override val presentableText: String? get() = text
    override val candidateSymbols: Collection<KaClassLikeSymbol> get() = emptyList()
    override val qualifiers: List<KaClassTypeQualifier> get() = emptyList()
    override val nullability get() = nullabilityOf(marked)
    override val abbreviation: KaUsualClassType? get() = null
    override val annotations: KaAnnotationList get() = KcEmptyAnnotations
    override fun withNullability(nullable: Boolean): KaType = KcErrorType(session, text, nullable)
    override val classNames: Set<String> get() = emptySet()
    override fun toString() = "<error: $text>"
}

internal class KcTypeArgument(private val session: KcSession, override val type: KaType, override val variance: Variance) :
    StubKaTypeArgumentWithVariance() {
    override val token get() = KcToken
    override fun equals(other: Any?) = other is KcTypeArgument && other.type == type && other.variance == variance
    override fun hashCode() = type.hashCode() * 3 + variance.ordinal
    override fun toString() = if (variance == Variance.INVARIANT) type.toString() else "${variance.label} $type"
}

internal class KcStarProjection(private val session: KcSession) : StubKaStarTypeProjection() {
    override val token get() = KcToken
    override fun equals(other: Any?) = other is KcStarProjection
    override fun hashCode() = 7
    override fun toString() = "*"
}

/** Type relations, as kc's facts tell them: by classifier names, ignoring type arguments. */
internal object KcTypeRelations {
    fun isSubtype(a: KaType, b: KaType): Boolean {
        if (a == b) return true
        val ab = a as? KcTypeBase ?: return false
        val bb = b as? KcTypeBase ?: return false
        if (a is KcErrorType || b is KcErrorType) return false
        if (ab.holdsNull && !bb.holdsNull && b !is KcFlexibleType) return false
        if (isNothing(a)) return true
        val target = when (b) {
            is KcClassType -> b.classId.asFqNameString()
            is KcFunctionType -> b.classId.asFqNameString()
            is KcFlexibleType -> return isSubtype(a.withNullability(false), b.lowerBound)
            is KcTypeParameterType -> return a is KcTypeParameterType && a.name == b.name
            else -> return false
        }
        if (target == "kotlin.Any") return true
        if (a is KcClassType && b is KcClassType && a.classId == b.classId) return argumentsConform(a, b)
        return target in ab.classNames
    }

    /**
     * Whether the arguments of [a] conform to those of [b], of the same
     * class, by the class's type parameters' variance: kc knows it for the
     * sources' classes and the standard library's collections, and takes
     * any arguments to conform for other classes.
     */
    private fun argumentsConform(a: KcClassType, b: KcClassType): Boolean {
        if (a.typeArguments.size != b.typeArguments.size) return true
        val declared = declaredVariance(a.session, a.classId, a.typeArguments.size) ?: return true
        for (i in a.typeArguments.indices) {
            val pb = b.typeArguments[i] as? KcTypeArgument ?: continue
            val pa = a.typeArguments[i] as? KcTypeArgument ?: return false
            val v = if (pb.variance != Variance.INVARIANT) pb.variance else declared[i]
            val ok = when (v) {
                Variance.OUT_VARIANCE -> isSubtype(pa.type, pb.type)
                Variance.IN_VARIANCE -> isSubtype(pb.type, pa.type)
                Variance.INVARIANT -> pa.variance == Variance.INVARIANT && isSubtype(pa.type, pb.type) && isSubtype(pb.type, pa.type)
            }
            if (!ok) return false
        }
        return true
    }

    private val covariant = setOf(
        "kotlin/collections/Iterable", "kotlin/collections/Collection", "kotlin/collections/List", "kotlin/collections/Set",
        "kotlin/collections/Iterator", "kotlin/collections/ListIterator", "kotlin/sequences/Sequence",
        "kotlin/Pair", "kotlin/Triple", "kotlin/Lazy", "kotlin/Result",
    )
    private val invariant = setOf(
        "kotlin/Array", "kotlin/collections/MutableIterable", "kotlin/collections/MutableCollection", "kotlin/collections/MutableList",
        "kotlin/collections/MutableSet", "kotlin/collections/MutableMap", "kotlin/collections/MutableIterator",
        "kotlin/collections/ArrayList", "kotlin/collections/HashMap", "kotlin/collections/LinkedHashMap",
        "kotlin/collections/HashSet", "kotlin/collections/LinkedHashSet",
    )

    private fun declaredVariance(session: KcSession, id: ClassId, n: Int): List<Variance>? {
        val s = id.asString()
        return when {
            s in covariant -> List(n) { Variance.OUT_VARIANCE }
            s in invariant -> List(n) { Variance.INVARIANT }
            s == "kotlin/collections/Map" && n == 2 -> listOf(Variance.INVARIANT, Variance.OUT_VARIANCE)
            s == "kotlin/Comparable" -> List(n) { Variance.IN_VARIANCE }
            else -> session.sourceClass(id)?.typeParameters?.takeIf { it.size == n }?.map { it.variance }
        }
    }

    /** As the Analysis API's: [a]'s classifier is [id]'s class or a subclass of it; Nothing isn't. */
    fun isSubtype(a: KaType, id: ClassId): Boolean {
        val ab = a as? KcTypeBase ?: return false
        val target = id.asFqNameString()
        return target == "kotlin.Any" || target in ab.classNames
    }

    fun isNothing(t: KaType) = (t as? KcClassType)?.classId?.asFqNameString() == "kotlin.Nothing"
}
