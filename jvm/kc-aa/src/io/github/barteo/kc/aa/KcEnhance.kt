package io.github.barteo.kc.aa

import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.symbols.KaCallableSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaFunctionSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.analysis.api.types.KaSubstitutor
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.types.KaTypeArgumentWithVariance
import org.jetbrains.kotlin.analysis.api.types.KaTypeProjection
import org.jetbrains.kotlin.builtins.jvm.JavaToKotlinClassMap
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.types.Variance

/**
 * A Java method's return and parameter types as FIR's signature
 * enhancement has them, null where they stay as declared: a flexible
 * type takes its nullability and its collection's mutability from the
 * members the method overrides. `AbstractList.equals(Object)` overrides
 * `Any.equals(other: Any?)`, so its parameter is `Any?`;
 * `AbstractMap.get` overrides `Map.get(key: K): V?` and returns `V?`.
 */
internal class KcEnhancement(val returnType: KaType?, val params: List<KaType?>)

/**
 * A type position's qualifiers, as FIR's JavaTypeQualifiers: [n] is true
 * for NULLABLE, false for NOT_NULL; [m] true for MUTABLE, false for
 * READ_ONLY; [dnn] makes a not-null type parameter's type `T & Any`.
 */
private data class Q(val n: Boolean? = null, val m: Boolean? = null, val dnn: Boolean = false)

private val NONE = Q()

/**
 * FIR's signature enhancement of Java methods (AbstractSignatureParts,
 * javaTypeUtils): each type is flattened into its positions, the type
 * and then its arguments' trees in order, and each position takes the
 * qualifiers its own annotations and the overridden members' types at
 * the same position agree on, or a predefined JDK signature's
 * (predefinedEnhancementInfo.kt). A Java type is flexible there before
 * enhancement, its annotations only qualifiers; kc's types already have
 * the nullability of a method's or parameter's annotation, so a type
 * that isn't flexible is taken back to its flexible one first.
 */
internal class KcEnhancer(private val round: KcRound) {
    private val rel get() = round.relations

    /**
     * [d]'s enhancement as a member of class [owner] that overrides
     * [supers] (or implements, a property, for the return type), each
     * with the substitutor of its class in [owner]'s supertypes; [erased] are the parameter types FIR gives parameters
     * Java erases (`KcMemberBuilder.erasedMatch`), which it enhances in
     * place of their own. Null if it has none.
     * While [owner]'s members are built, [d]'s types are its declared ones.
     * With [staticOnly], only a predefined signature enhances it.
     */
    fun enhance(
        owner: KcClassSymbol, d: KcFunctionSymbol, supers: List<Pair<KaCallableSymbol, KaSubstitutor>>, erased: Map<Int, KaType>,
        staticOnly: Boolean = false,
    ): KcEnhancement? {
        val predefined = predefined(owner, d)
        if (staticOnly && predefined == null) return null
        val params = d.valueParameters
        if (supers.isEmpty() && predefined == null && erased.isEmpty() &&
            !annotated(d.returnType) && params.none { annotated(it.returnType) }
        ) return null
        val ret = enhanceType(
            d.returnType, d.annotations, supers.map { (s, sub) -> sub.substitute(s.returnType) },
            covariant = true, predefined?.ret,
        )
        val ps = params.mapIndexed { i, p ->
            // FIR enhances flexible positions only: an erased parameter
            // implementing MutableMap<K, V>.get keeps its K.
            val e = erased[i]
            if (e != null && e !is KcFlexibleType) return@mapIndexed e
            enhanceType(
                e ?: p.returnType, p.annotations,
                supers.mapNotNull { (s, sub) -> (s as? KaFunctionSymbol)?.valueParameters?.getOrNull(i)?.let { sub.substitute(it.returnType) } },
                covariant = false, predefined?.params?.getOrNull(i),
            ) ?: e
        }
        return if (ret == null && ps.all { it == null }) null else KcEnhancement(ret, ps)
    }

    /**
     * Own type [own] enhanced with the overridden members' types [supers]
     * at its position, or null if that doesn't change it. Positions below
     * a return type's top one take nothing from overridden members whose
     * type isn't equal to it, as FIR's: `ArrayList<String>` overriding
     * `List<String?>` keeps its argument.
     */
    private fun enhanceType(own: KaType, container: Iterable<KaAnnotation>, supers: List<KaType>, covariant: Boolean, predefined: Map<Int, Q>?): KaType? {
        val original = flexible(own) ?: return null
        val tree = flatten(own)
        val superTrees = supers.map(::flatten)
        val different = covariant && supers.any { !rel.equal(original, it) }
        val qs = tree.indices.map { i ->
            predefined?.get(i) ?: if (i > 0 && different) NONE else combine(
                ownQualifiers(tree[i], if (i == 0) container else emptyList(), i == 0),
                superTrees.mapNotNull { t -> t.getOrNull(i)?.let(::superQualifiers) },
                covariant && i == 0,
            )
        }
        val r = Cursor(qs).tree(own)
        return if (r == own) null else r
    }

    /** FIR's computeQualifiersForOverride: [own] and the overridden members' [supers] qualifiers at a position. */
    private fun combine(own: Q, supers: List<Q>, covariant: Boolean): Q {
        val n = select(supers.mapNotNull { it.n }.toSet(), own.n, covariant, low = false, high = true)
        val m = select(supers.mapNotNull { it.m }.toSet(), own.m, covariant, low = true, high = false)
        return Q(n, m, n == false && (own.dnn || supers.any { it.dnn }))
    }

    /**
     * FIR's select: a covariant position prefers [low], the stronger
     * qualifier, but one whose own is [high] while a supertype's is [low]
     * gets none; an invariant one only a qualifier they all agree on.
     */
    private fun select(set: Set<Boolean>, own: Boolean?, covariant: Boolean, low: Boolean, high: Boolean): Boolean? {
        if (covariant) {
            val sup = if (low in set) low else if (high in set) high else null
            return if (sup == low && own == high) null else own ?: sup
        }
        return (if (own == null) set else set + own).singleOrNull()
    }

    /**
     * The qualifiers of own type [t] at a position: its annotations', the
     * [container]'s for the top one; kc's type has those of a method's or
     * a parameter's nullability annotation already.
     */
    private fun ownQualifiers(t: KaType?, container: Iterable<KaAnnotation>, top: Boolean): Q {
        if (t == null) return NONE
        val m = mutability(t.annotations + container)
        val annotated = nullability(t.annotations)
        val n = annotated ?: when {
            t is KcFlexibleType -> null
            t is KcDefinitelyNotNullType -> false
            // javac puts a TYPE_USE annotation of `@NotNull String[] f()`
            // on the element type, for which FIR ignores the method's.
            top && isArray(t) && (t as KcBaseClassType).typeArguments.firstOrNull()?.type?.let { nullability(it.annotations) } != null -> null
            t is KcTypeBase -> t.marked
            else -> null
        }
        val param = t is KcTypeParameterType || t is KcDefinitelyNotNullType ||
            t is KcFlexibleType && t.lowerBound.let { it is KcTypeParameterType || it is KcDefinitelyNotNullType }
        // FIR makes a type parameter's type that an annotation makes not null `T & Any`.
        return Q(n, m, n == false && param && (annotated == false || t !is KcFlexibleType))
    }

    /** FIR's extractQualifiers: the qualifiers an overridden member's type [t] gives a position. */
    private fun superQualifiers(t: KaType?): Q? {
        if (t == null) return null
        val lo = if (t is KcFlexibleType) t.lowerBound else t
        val up = if (t is KcFlexibleType) t.upperBound else t
        val n = when {
            marked(lo) -> true
            !marked(up) -> false
            else -> null
        }
        val m = when {
            classId(lo) in readOnlyIds -> false
            classId(up) in KcTypeContext.readOnly.keys -> true
            else -> null
        }
        return Q(n, m, t is KcDefinitelyNotNullType)
    }

    private fun marked(t: KaType) = t !is KcDefinitelyNotNullType && (t as? KcTypeBase)?.marked == true

    private fun classId(t: KaType) = (t as? KcBaseClassType)?.classId?.asString()

    /** Whether type [t] or one of its arguments' has a nullability or mutability annotation, or kc's nullability for one. */
    private fun annotated(t: KaType): Boolean =
        flatten(t).any { it != null && (nullability(it.annotations) != null || mutability(it.annotations) != null) } ||
            t !is KcFlexibleType && flexible(t) != null

    /**
     * Type [t] as FIR has it before enhancement: flexible, unless it is a
     * primitive's. Null if it isn't a type enhancement changes.
     */
    private fun flexible(t: KaType): KcFlexibleType? = when (t) {
        is KcFlexibleType -> t
        is KcDefinitelyNotNullType -> (t.original as? KcTypeBase)?.let { KcFlexibleType(round, it.withNullability(false), it.withNullability(true)) }
        is KcTypeParameterType -> KcFlexibleType(round, t.withNullability(false), t.withNullability(true))
        is KcBaseClassType -> {
            val id = t.classId.asString()
            if (!t.marked && id in primitives) null
            else {
                val upper = when {
                    id in KcTypeContext.readOnly -> t.rebuild(ClassId.fromString(KcTypeContext.readOnly.getValue(id)), t.typeArguments, true)
                    id == ARRAY -> t.rebuild(t.classId, t.typeArguments.map(::out), true)
                    else -> t.withNullability(true)
                }
                KcFlexibleType(round, t.withNullability(false), upper)
            }
        }
        else -> null
    }

    /** Type [t] as FIR has it before enhancement (see [flexible]), or [t] if enhancement doesn't change it. */
    fun flexibleOf(t: KaType): KaType = flexible(t) ?: t

    /**
     * Type [t] flexible at every position, as [flexibleOf] makes its top:
     * FIR matches a Java method's overrides ignoring nullability, so
     * `BiFunction<in V!, ...>` overrides `BiFunction<in V & Any, ...>`.
     */
    fun deepFlexibleOf(t: KaType): KaType = when (val f = flexible(t)) {
        null -> t
        else -> KcFlexibleType(round, deepArgs(f.lowerBound), deepArgs(f.upperBound))
    }

    private fun deepArgs(t: KaType): KaType = if (t !is KcBaseClassType || t.typeArguments.isEmpty()) t else t.rebuild(
        t.classId,
        t.typeArguments.map { a -> if (a is KaTypeArgumentWithVariance) KcTypeArgument(deepFlexibleOf(a.type), a.variance, round.token) else a },
        t.marked,
    )

    private fun out(a: KaTypeProjection): KaTypeProjection =
        if (a is KaTypeArgumentWithVariance && a.variance == Variance.INVARIANT) KcTypeArgument(a.type, Variance.OUT_VARIANCE, round.token) else a

    private fun isArray(t: KaType) = t is KcBaseClassType && t.classId.asString() == ARRAY

    /** Applies a position's qualifiers to each position of a type, in [flatten]'s order. */
    private inner class Cursor(private val qs: List<Q>) {
        private var i = 0

        /** [t], an own type or an argument's, enhanced. */
        fun tree(t: KaType): KaType {
            val q = qs.getOrElse(i) { NONE }
            i++
            val f = flexible(t)
            if (f == null) {
                i += flatten(t).size - 1
                return t
            }
            val start = i
            val lo = bound(f.lowerBound, q, lower = true)
            val end = i
            i = start
            val up = bound(f.upperBound, q, lower = false)
            i = end
            // FIR's coneFlexibleOrSimpleType: equal bounds make one type.
            return if (lo == up) lo else KcFlexibleType(round, lo, up)
        }

        private fun bound(b: KaType, q: Q, lower: Boolean): KaType = when (b) {
            is KcDefinitelyNotNullType -> {
                val e = bound(b.original, q, lower)
                if (q.n == null && e is KcTypeParameterType && !e.marked) KcDefinitelyNotNullType(round, e) else e
            }
            is KcBaseClassType -> {
                val id = b.classId.asString()
                // FIR's enhanceMutability: a read-only lower bound, a mutable upper one.
                val to = when (q.m) {
                    false -> if (lower) KcTypeContext.readOnly[id] else null
                    true -> if (!lower) mutableIds[id] else null
                    null -> null
                }
                val args = b.typeArguments.map { a ->
                    val at = a.type
                    if (at == null) {
                        i++
                        a
                    } else {
                        val e = tree(at)
                        if (e === at) a else KcTypeArgument(e, (a as KaTypeArgumentWithVariance).variance, round.token)
                    }
                }
                b.rebuild(if (to == null) b.classId else ClassId.fromString(to), args, q.n ?: b.marked)
            }
            is KcTypeParameterType -> {
                val n = q.n ?: b.marked
                val r = b.withNullability(n)
                if (q.dnn && !n) KcDefinitelyNotNullType(round, r) else r
            }
            else -> {
                i += flatten(b).size - 1
                b
            }
        }
    }

    /** The predefined enhancement of JDK method [d], a member of [owner] (a Kotlin collection's for one it maps). */
    private fun predefined(owner: KcClassSymbol, d: KcFunctionSymbol): KcPredefined? {
        val id = owner.classId ?: return null
        val java = JavaToKotlinClassMap.mapKotlinToJava(id.asSingleFqName().toUnsafe()) ?: id
        val cls = java.packageFqName.asString().replace('.', '/') + "/" + java.relativeClassName.asString().replace('.', '$')
        if (predefinedClasses[cls]?.contains(d.name.asString()) != true) return null
        if (d.valueParameters.any { it.isVararg }) return null
        val types = KcJvmTypes(round)
        val ret = types.descriptor(d.returnType).let { if (it == "Lkotlin/Unit;") "V" else it }
        return PREDEFINED[cls + "." + d.name.asString() + d.valueParameters.joinToString("", "(", ")") { types.descriptor(it.returnType) } + ret]
    }

    companion object {
        private const val ARRAY = "kotlin/Array"

        private val readOnlyIds = KcTypeContext.readOnly.values.toSet()
        private val mutableIds = KcTypeContext.readOnly.entries.associate { (k, v) -> v to k }

        private val primitives = setOf(
            "kotlin/Int", "kotlin/Long", "kotlin/Short", "kotlin/Byte", "kotlin/Char", "kotlin/Boolean",
            "kotlin/Float", "kotlin/Double", "kotlin/Unit",
        )

        private val notNull = listOf(
            "org/jetbrains/annotations/NotNull", "androidx/annotation/NonNull", "androidx/annotation/RecentlyNonNull",
            "android/support/annotation/NonNull", "android/annotation/NonNull", "com/android/annotations/NonNull",
            "org/eclipse/jdt/annotation/NonNull", "org/checkerframework/checker/nullness/qual/NonNull",
            "org/checkerframework/checker/nullness/compatqual/NonNullDecl", "edu/umd/cs/findbugs/annotations/NonNull",
            "lombok/NonNull", "io/reactivex/annotations/NonNull", "io/reactivex/rxjava3/annotations/NonNull",
            "org/jspecify/annotations/NonNull", "org/jspecify/nullness/NonNull",
        )
        private val nullable = listOf(
            "org/jetbrains/annotations/Nullable", "androidx/annotation/Nullable", "androidx/annotation/RecentlyNullable",
            "android/support/annotation/Nullable", "android/annotation/Nullable", "com/android/annotations/Nullable",
            "org/eclipse/jdt/annotation/Nullable", "org/checkerframework/checker/nullness/qual/Nullable",
            "org/checkerframework/checker/nullness/compatqual/NullableDecl", "edu/umd/cs/findbugs/annotations/Nullable",
            "edu/umd/cs/findbugs/annotations/CheckForNull", "edu/umd/cs/findbugs/annotations/PossiblyNull",
            "io/reactivex/annotations/Nullable", "io/reactivex/rxjava3/annotations/Nullable",
            "org/jspecify/annotations/Nullable", "org/jspecify/nullness/Nullable",
        )
        private val nullabilityByName = notNull.associateWith { false } + nullable.associateWith { true }
        private val mutabilityByName = mapOf(
            "org/jetbrains/annotations/ReadOnly" to false, "kotlin/annotations/jvm/ReadOnly" to false,
            "org/jetbrains/annotations/Mutable" to true, "kotlin/annotations/jvm/Mutable" to true,
        )

        /** The nullability annotations [anns] agree on: true for nullable, null for none or both. */
        private fun nullability(anns: Iterable<KaAnnotation>): Boolean? =
            anns.mapNotNull { a -> a.classId?.asString()?.let(nullabilityByName::get) }.toSet().singleOrNull()

        private fun mutability(anns: Iterable<KaAnnotation>): Boolean? =
            anns.mapNotNull { a -> a.classId?.asString()?.let(mutabilityByName::get) }.toSet().singleOrNull()

        /** The positions of type [t] in FIR's order: [t], then each argument's, a star projection's null. */
        private fun flatten(t: KaType?, out: MutableList<KaType?> = ArrayList()): List<KaType?> {
            out += t
            val args = when (t) {
                is KcFlexibleType -> (t.lowerBound as? KcBaseClassType)?.typeArguments
                is KcDefinitelyNotNullType -> null
                is KcBaseClassType -> t.typeArguments
                else -> null
            }
            args?.forEach { flatten(it.type, out) }
            return out
        }
    }
}

/** A predefined JDK signature's qualifiers by position, of its return type and of each parameter's. */
private class KcPredefined(val ret: Map<Int, Q>?, val params: List<Map<Int, Q>?>)

/** NOT_PLATFORM, NULLABLE and NOT_NULLABLE (definitely not null), as in predefinedEnhancementInfo.kt. */
private val P = Q(false)
private val N = Q(true)
private val D = Q(false, dnn = true)
private const val OBJ = "Ljava/lang/Object;"

private class PredefinedBuilder {
    val map = HashMap<String, KcPredefined>()

    fun fn(cls: String, name: String, ret: String, retQ: List<Q>?, vararg params: Pair<String, List<Q>?>) {
        val key = cls + "." + name + params.joinToString("", "(", ")") { it.first } + ret
        val q = { l: List<Q>? -> l?.withIndex()?.associate { (i, v) -> i to v } }
        map[key] = KcPredefined(q(retQ), params.map { q(it.second) })
    }
}

private fun f(name: String) = "Ljava/util/function/$name;"

/**
 * FIR's PREDEFINED_FUNCTION_ENHANCEMENT_INFO_BY_SIGNATURE, by JVM class,
 * name and descriptor. The ones whose errors start at a language
 * version, 2.1 or 2.2, are taken as errors, as at 2.4.
 */
private val PREDEFINED: Map<String, KcPredefined> = PredefinedBuilder().apply {
    val c = "java/util/Collection"
    val m = "java/util/Map"
    val o = "java/util/Optional"
    fn("java/util/Iterator", "forEachRemaining", "V", null, f("Consumer") to listOf(P, P))
    fn("java/lang/Iterable", "spliterator", "Ljava/util/Spliterator;", listOf(P, P))
    fn(c, "removeIf", "Z", null, f("Predicate") to listOf(P, P))
    fn(c, "stream", "Ljava/util/stream/Stream;", listOf(P, P))
    fn(c, "parallelStream", "Ljava/util/stream/Stream;", listOf(P, P))
    fn("java/util/List", "replaceAll", "V", null, f("UnaryOperator") to listOf(P, P))
    for (l in listOf("java/util/List", "java/util/LinkedList", "java/util/LinkedHashSet")) {
        fn(l, "addFirst", "V", null, OBJ to listOf(P))
        fn(l, "addLast", "V", null, OBJ to listOf(P))
        fn(l, "removeFirst", OBJ, listOf(P))
        fn(l, "removeLast", OBJ, listOf(P))
    }
    fn("java/util/LinkedHashSet", "getFirst", OBJ, listOf(P))
    fn("java/util/LinkedHashSet", "getLast", OBJ, listOf(P))
    fn(m, "forEach", "V", null, f("BiConsumer") to listOf(P, P, P))
    fn(m, "putIfAbsent", OBJ, listOf(N), OBJ to listOf(P), OBJ to listOf(P))
    fn(m, "replace", OBJ, listOf(N), OBJ to listOf(P), OBJ to listOf(P))
    fn(m, "replace", "Z", null, OBJ to listOf(P), OBJ to listOf(P), OBJ to listOf(P))
    fn(m, "replaceAll", "V", null, f("BiFunction") to listOf(P, P, P, P))
    fn(m, "compute", OBJ, listOf(N), OBJ to listOf(P), f("BiFunction") to listOf(P, P, N, N))
    fn(m, "computeIfAbsent", OBJ, listOf(P), OBJ to listOf(P), f("Function") to listOf(P, P, P))
    fn(m, "computeIfPresent", OBJ, listOf(N), OBJ to listOf(P), f("BiFunction") to listOf(P, P, D, N))
    fn(m, "merge", OBJ, listOf(N), OBJ to listOf(P), OBJ to listOf(D), f("BiFunction") to listOf(P, D, D, N))
    fn("java/util/LinkedHashMap", "putFirst", OBJ, listOf(N), OBJ to listOf(P), OBJ to listOf(P))
    fn("java/util/LinkedHashMap", "putLast", OBJ, listOf(N), OBJ to listOf(P), OBJ to listOf(P))
    fn(o, "empty", "L$o;", listOf(P, D))
    fn(o, "of", "L$o;", listOf(P, D), OBJ to listOf(D))
    fn(o, "ofNullable", "L$o;", listOf(P, D), OBJ to listOf(N))
    fn(o, "get", OBJ, listOf(D))
    fn(o, "ifPresent", "V", null, f("Consumer") to listOf(P, D))
    fn("java/lang/ref/Reference", "get", OBJ, listOf(N))
    fn("java/util/function/Predicate", "test", "Z", null, OBJ to listOf(P))
    fn("java/util/function/BiPredicate", "test", "Z", null, OBJ to listOf(P), OBJ to listOf(P))
    fn("java/util/function/Consumer", "accept", "V", null, OBJ to listOf(P))
    fn("java/util/function/BiConsumer", "accept", "V", null, OBJ to listOf(P), OBJ to listOf(P))
    fn("java/util/function/Function", "apply", OBJ, listOf(P), OBJ to listOf(P))
    fn("java/util/function/BiFunction", "apply", OBJ, listOf(P), OBJ to listOf(P), OBJ to listOf(P))
    fn("java/util/function/Supplier", "get", OBJ, listOf(P))
}.map

/** The names of the methods [PREDEFINED] has, by JVM class. */
private val predefinedClasses: Map<String, Set<String>> =
    PREDEFINED.keys.groupBy({ it.substringBefore('.') }, { it.substringAfter('.').substringBefore('(') }).mapValues { it.value.toSet() }

// kc's class types are KaClassTypes.
private val KcBaseClassType.classId: ClassId get() = (this as KaClassType).classId
private val KcBaseClassType.typeArguments: List<KaTypeProjection> get() = (this as KaClassType).typeArguments
