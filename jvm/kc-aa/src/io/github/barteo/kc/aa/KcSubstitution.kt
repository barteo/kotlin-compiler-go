package io.github.barteo.kc.aa

import org.jetbrains.kotlin.analysis.api.signatures.KaCallableSignature
import org.jetbrains.kotlin.analysis.api.signatures.KaFunctionSignature
import org.jetbrains.kotlin.analysis.api.signatures.KaVariableSignature
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.types.*
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.Variance

/**
 * Replaces type parameters' types by [map]'s types, in one pass: a type
 * that replaces one isn't substituted again. A replaced `T?` is made
 * nullable, a replaced `T & Any` not null.
 */
internal class KcSubstitutor(private val round: KcRound, val map: Map<KaTypeParameterSymbol, KaType>) : KaSubstitutor {
    override val token get() = round.token

    override fun substituteOrNull(type: KaType): KaType? = when (type) {
        is KcTypeParameterType -> map[type.symbol]?.let { if (type.marked) nullable(it) else it }
        is KcClassType -> args(type.typeArguments)?.let { KcClassType(round, type.classId, it, type.marked) }
        is KcFunctionType -> args(type.typeArguments)?.let {
            KcFunctionType(round, type.classId, it, type.marked, type.hasReceiver, type.contexts)
        }
        is KcFlexibleType -> {
            val lo = substituteOrNull(type.lowerBound)
            val hi = substituteOrNull(type.upperBound)
            if (lo == null && hi == null) null
            else KcFlexibleType(round, flexibleBound(lo ?: type.lowerBound, false), flexibleBound(hi ?: type.upperBound, true))
        }
        is KcDefinitelyNotNullType -> substituteOrNull(type.original)?.let { notNull(it) }
        else -> null
    }

    private fun args(args: List<KaTypeProjection>): List<KaTypeProjection>? {
        var changed = false
        val out = args.map { a ->
            if (a is KcTypeArgument) {
                substituteOrNull(a.type)?.let { changed = true; KcTypeArgument(it, a.variance, a.token) } ?: a
            } else a
        }
        return if (changed) out else null
    }

    /** A flexible type's bound: a flexible replacement's lower or upper bound. */
    private fun flexibleBound(t: KaType, upper: Boolean): KaType =
        if (t is KcFlexibleType) (if (upper) t.upperBound else t.lowerBound) else t

    private fun nullable(t: KaType): KaType = (t as? KcTypeBase)?.withNullability(true) ?: t

    private fun notNull(t: KaType): KaType = when (t) {
        is KcTypeParameterType -> KcDefinitelyNotNullType(round, t.withNullability(false))
        is KcFlexibleType -> notNull(t.lowerBound)
        is KcTypeBase -> t.withNullability(false)
        else -> t
    }

    override fun equals(other: Any?) = other is KcSubstitutor && other.map == map
    override fun hashCode() = map.hashCode()
    override fun toString() = "KcSubstitutor$map"
}

/** Substitutes with [first], then with [second]. */
internal class KcChainedSubstitutor(private val first: KaSubstitutor, private val second: KaSubstitutor) : KaSubstitutor {
    override val token get() = first.token
    override fun substituteOrNull(type: KaType): KaType? {
        val a = first.substituteOrNull(type)
        return second.substituteOrNull(a ?: type) ?: a
    }
}

/** A function's signature: its symbol with its types, substituted. */
internal class KcFunctionSignature<S : KaFunctionSymbol>(
    private val round: KcRound,
    override val symbol: S,
    override val returnType: KaType,
    override val receiverType: KaType?,
    override val valueParameters: List<KaVariableSignature<KaValueParameterSymbol>>,
) : KaFunctionSignature<S> {
    override val token get() = round.token
    override fun substitute(substitutor: KaSubstitutor): KaFunctionSignature<S> = KcFunctionSignature(
        round, symbol, substitutor.substitute(returnType), receiverType?.let(substitutor::substitute),
        valueParameters.map { it.substitute(substitutor) },
    )
    override fun equals(other: Any?) = other is KcFunctionSignature<*> && other.symbol == symbol &&
        other.returnType == returnType && other.receiverType == receiverType && other.valueParameters == valueParameters
    override fun hashCode() = symbol.hashCode() * 31 + returnType.hashCode()
    override fun toString() = "$symbol${valueParameters.joinToString(", ", "(", ")")}: $returnType"
}

/** A variable's signature: its symbol with its type, substituted. */
internal class KcVariableSignature<S : KaVariableSymbol>(
    private val round: KcRound,
    override val symbol: S,
    override val returnType: KaType,
    override val receiverType: KaType?,
) : KaVariableSignature<S> {
    override val token get() = round.token
    override val name: Name get() = symbol.name
    override fun substitute(substitutor: KaSubstitutor): KaVariableSignature<S> =
        KcVariableSignature(round, symbol, substitutor.substitute(returnType), receiverType?.let(substitutor::substitute))
    override fun equals(other: Any?) = other is KcVariableSignature<*> && other.symbol == symbol &&
        other.returnType == returnType && other.receiverType == receiverType
    override fun hashCode() = symbol.hashCode() * 31 + returnType.hashCode()
    override fun toString() = "$name: $returnType"
}

/** Substitution, subtyping and supertypes over kc's types. */
internal class KcTypeRelations(private val round: KcRound) {
    val empty: KaSubstitutor by lazy { KaSubstitutor.Empty(round.token) }

    fun substitutor(map: Map<KaTypeParameterSymbol, KaType>): KaSubstitutor =
        if (map.isEmpty()) empty else KcSubstitutor(round, map)

    /**
     * The substitutor of the class type [t]'s class's type parameters by
     * its arguments; a star becomes the parameter's first bound.
     */
    fun typeSubstitutor(t: KaClassType): KaSubstitutor {
        val params = (t.symbol as? KaClassSymbol)?.typeParameters.orEmpty()
        if (params.isEmpty() || params.size != t.typeArguments.size) return empty
        return substitutor(params.zip(t.typeArguments).associate { (p, a) ->
            p to (a.type ?: p.upperBounds.firstOrNull() ?: anyType(true))
        })
    }

    fun anyType(nullable: Boolean) = KcClassType(round, ANY, marked = nullable)

    /** The class type a type stands for in supertype walks: a flexible type's lower bound. */
    fun classType(t: KaType): KaClassType? = when (t) {
        is KcFlexibleType -> classType(t.lowerBound)
        is KaClassType -> t
        else -> null
    }

    /** The supertypes of class type [t]'s class, in [t]'s arguments. */
    fun directSupertypes(t: KaType): List<KaType> {
        val c = classType(t) ?: return emptyList()
        val s = c.symbol as? KaClassSymbol ?: return emptyList()
        val sub = typeSubstitutor(c)
        return s.superTypes.map { sub.substitute(it) }
    }

    /** [t] seen as a type of class [id] (t itself if it is), or null if [id] isn't among its supertypes. */
    fun asSuper(t: KaType, id: ClassId): KaClassType? {
        val start = classType(t) ?: return null
        if (start.classId == id) return start
        val seen = HashSet<ClassId>()
        val queue = ArrayDeque<KaClassType>()
        queue.add(start)
        seen.add(start.classId)
        while (queue.isNotEmpty()) {
            for (s in directSupertypes(queue.removeFirst())) {
                val c = classType(s) ?: continue
                if (c.classId == id) return c
                if (seen.add(c.classId)) queue.add(c)
            }
        }
        return null
    }

    fun isNullable(t: KaType): Boolean = when (t) {
        is KcFlexibleType -> isNullable(t.upperBound)
        is KcDefinitelyNotNullType -> false
        is KcTypeParameterType -> t.marked || t.symbol.upperBounds.all(::isNullable)
        is KcTypeBase -> t.marked
        else -> false
    }

    fun isSubtype(a: KaType, b: KaType, lenient: Boolean): Boolean {
        if (a is KaErrorType || b is KaErrorType) return lenient
        if (a is KcFlexibleType) return isSubtype(a.lowerBound, b, lenient)
        if (b is KcFlexibleType) return isSubtype(a, b.upperBound, lenient)
        if (isNullable(a) && !isNullable(b)) return false
        return structural(a, b, lenient)
    }

    /** Whether [a] is a subtype of [b] if nullability is ignored. */
    private fun structural(a: KaType, b: KaType, lenient: Boolean): Boolean {
        if (b is KaClassType && b.classId == ANY) return true
        if (a is KaClassType && a.classId == NOTHING) return true
        if (a is KcDefinitelyNotNullType) return structural(a.original, b, lenient)
        if (b is KcDefinitelyNotNullType) return structural(a, b.original, lenient)
        if (a is KcTypeParameterType) {
            if (b is KcTypeParameterType && b.symbol == a.symbol) return true
            return a.symbol.upperBounds.any { bound ->
                val ub = if (bound is KcFlexibleType) bound.upperBound else bound
                ub !is KaErrorType && structural(ub, b, lenient)
            }
        }
        if (b is KcTypeParameterType) return false
        if (b !is KaClassType || a !is KaClassType) return false
        val sup = asSuper(a, b.classId) ?: return false
        if (b.typeArguments.isEmpty()) return true
        if (sup.typeArguments.size != b.typeArguments.size) return false
        val params = (b.symbol as? KaClassSymbol)?.typeParameters.orEmpty()
        return sup.typeArguments.indices.all { i ->
            argument(sup.typeArguments[i], b.typeArguments[i], params.getOrNull(i)?.variance ?: Variance.INVARIANT, lenient)
        }
    }

    /** Whether argument [a] fits argument [b] of a parameter of declaration-site variance [dv]. */
    private fun argument(a: KaTypeProjection, b: KaTypeProjection, dv: Variance, lenient: Boolean): Boolean {
        val bt = b.type ?: return true
        val bv = (b as? KaTypeArgumentWithVariance)?.variance ?: Variance.INVARIANT
        val effective = if (bv != Variance.INVARIANT) bv else dv
        val at = a.type ?: return effective == Variance.OUT_VARIANCE && isSubtype(anyType(true), bt, lenient)
        val av = (a as? KaTypeArgumentWithVariance)?.variance ?: Variance.INVARIANT
        return when (effective) {
            Variance.OUT_VARIANCE -> av != Variance.IN_VARIANCE && isSubtype(at, bt, lenient)
            Variance.IN_VARIANCE -> av != Variance.OUT_VARIANCE && isSubtype(bt, at, lenient)
            Variance.INVARIANT -> av == Variance.INVARIANT && isSubtype(at, bt, lenient) && isSubtype(bt, at, lenient)
        }
    }

    fun equal(a: KaType, b: KaType, lenient: Boolean = false) =
        a == b || isSubtype(a, b, lenient) && isSubtype(b, a, lenient)

    /** Whether a supertype of [t], or [t], is of class [id]. */
    fun isSubtype(t: KaType, id: ClassId, lenient: Boolean): Boolean = when (t) {
        is KaErrorType -> lenient
        is KcFlexibleType -> isSubtype(t.lowerBound, id, lenient)
        is KcDefinitelyNotNullType -> isSubtype(t.original, id, lenient)
        is KcTypeParameterType -> t.symbol.upperBounds.any { isSubtype(it, id, lenient) }
        is KaClassType -> t.classId == NOTHING || id == ANY || asSuper(t, id) != null
        else -> false
    }

    /** Whether class [sub] has [sup] among its supertypes, at any depth. */
    fun isSubClass(sub: KaClassSymbol, sup: KaClassSymbol): Boolean {
        val id = sup.classId ?: return false
        if (sub.classId == id) return false
        val seen = HashSet<ClassId>()
        val queue = ArrayDeque<KaClassSymbol>()
        queue.add(sub)
        while (queue.isNotEmpty()) {
            for (s in queue.removeFirst().superTypes) {
                val c = classType(s) ?: continue
                if (c.classId == id) return true
                if (seen.add(c.classId)) (c.symbol as? KaClassSymbol)?.let(queue::add)
            }
        }
        return false
    }

    fun isDirectSubClass(sub: KaClassSymbol, sup: KaClassSymbol): Boolean {
        val id = sup.classId ?: return false
        return sub.classId != id && sub.superTypes.any { classType(it)?.classId == id }
    }

    /**
     * The substitutor of [sup]'s type parameters by the types [sub] gives
     * them, with [sub]'s own type parameters, or null if [sup] isn't a
     * supertype of [sub].
     */
    fun inheritanceSubstitutor(sub: KaClassSymbol, sup: KaClassSymbol): KaSubstitutor? {
        val id = sup.classId ?: return null
        val self = selfType(sub) ?: return null
        return asSuper(self, id)?.let(::typeSubstitutor)
    }

    fun selfType(c: KaClassSymbol): KaClassType? {
        val id = c.classId ?: return null
        return KcClassType(round, id, c.typeParameters.map {
            KcTypeArgument(KcTypeParameterType(round, it.name, it, false), Variance.INVARIANT, round.token)
        })
    }

    fun <S : KaFunctionSymbol> functionSignature(s: S): KcFunctionSignature<S> = KcFunctionSignature(
        round, s, s.returnType, s.receiverType,
        s.valueParameters.map { KcVariableSignature(round, it, it.returnType, null) },
    )

    fun <S : KaVariableSymbol> variableSignature(s: S): KcVariableSignature<S> =
        KcVariableSignature(round, s, s.returnType, s.receiverType)

    @Suppress("UNCHECKED_CAST")
    fun <S : KaCallableSymbol> signature(s: S): KaCallableSignature<S> = when (s) {
        is KaFunctionSymbol -> functionSignature(s) as KaCallableSignature<S>
        is KaVariableSymbol -> variableSignature(s) as KaCallableSignature<S>
        else -> KcAnalysis.unsupported("signature of $s")
    }

    companion object {
        val ANY = ClassId.fromString("kotlin/Any")
        val NOTHING = ClassId.fromString("kotlin/Nothing")
    }
}
