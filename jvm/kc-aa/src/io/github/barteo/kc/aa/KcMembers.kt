@file:Suppress("DEPRECATION", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.aa

import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.base.KaContextReceiver
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.pointers.KaSymbolPointer
import org.jetbrains.kotlin.analysis.api.types.KaStarTypeProjection
import org.jetbrains.kotlin.analysis.api.types.KaSubstitutor
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.descriptors.Visibility
import org.jetbrains.kotlin.descriptors.java.JavaVisibilities
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name
import java.util.IdentityHashMap

/**
 * The members of a class as its member scope has them: [callables], its
 * own non-static ones first, then those it inherits, one per signature
 * (fake overrides where a supertype's arguments change their types, or
 * where several supertypes' members meet); [overridden], each declared
 * member's directly overridden declarations; and [statics], its static
 * members with those of its Java superclasses.
 */
internal class KcClassMembers(
    val callables: List<KaCallableSymbol>,
    val overridden: IdentityHashMap<KaCallableSymbol, List<KaCallableSymbol>>,
    val statics: List<KaDeclarationSymbol>,
    /** A Java class's declarations as FIR has them (see [KcMemberBuilder.javaDeclared]), or null. */
    val declared: List<KaDeclarationSymbol>? = null,
    /** Its Java methods' types as FIR enhances them from the members they override (see [KcEnhancement]). */
    val enhanced: IdentityHashMap<KaCallableSymbol, KcEnhancement> = IdentityHashMap(),
) {
    companion object {
        val EMPTY = KcClassMembers(emptyList(), IdentityHashMap(), emptyList())
    }
}

/** A member a class inherits from [original], a supertype's member, under a new signature. */
internal interface KcFakeOverride {
    /** The class the fake override is a member of. */
    val owner: KcClassSymbol
    val original: KaCallableSymbol
    /** The members an intersection override overrides, or empty. */
    val intersected: List<KaCallableSymbol>
}

/** The declarations [s] overrides or stands for: itself unless it is a fake override. */
internal fun declaredOriginals(s: KaCallableSymbol): List<KaCallableSymbol> = when (s) {
    is KcFakeOverride -> if (s.intersected.isNotEmpty()) s.intersected.flatMap(::declaredOriginals).distinct()
    else declaredOriginals(s.original)
    else -> listOf(s)
}

/** The declaration a fake override comes from, or [s] itself. */
internal fun fakeOverrideOriginal(s: KaCallableSymbol): KaCallableSymbol =
    if (s is KcFakeOverride) fakeOverrideOriginal(s.original) else s

/** The compiler's visibility for [v], which the Analysis API interfaces don't expose. */
internal fun compilerVisibilityOf(v: KaSymbolVisibility): Visibility = when (v) {
    KaSymbolVisibility.PRIVATE -> Visibilities.Private
    KaSymbolVisibility.PROTECTED -> Visibilities.Protected
    KaSymbolVisibility.INTERNAL -> Visibilities.Internal
    KaSymbolVisibility.PACKAGE_PRIVATE -> JavaVisibilities.PackageVisibility
    KaSymbolVisibility.LOCAL -> Visibilities.Local
    else -> Visibilities.Public
}

/** Whether a member belongs in the static scope (see `KcAnalysis.isStatic`). */
internal fun isStaticMember(s: KaDeclarationSymbol) = when (s) {
    is KcClassSymbol -> !s.isInner
    is KcEnumEntrySymbol -> true
    is KcFunctionSymbol -> s.isStatic
    is KcPropertySymbol -> s.isStatic
    is KcJavaFieldSymbol -> s.isStatic
    else -> false
}

/** Computes the [KcClassMembers] of class [c]. */
internal class KcMemberBuilder(private val round: KcRound, private val c: KcClassSymbol) {
    private val rel = round.relations
    private val enhancer = KcEnhancer(round)

    /** A member of a supertype and the substitutor of that supertype's arguments. */
    private class Candidate(val symbol: KaCallableSymbol, val sub: KaSubstitutor)

    fun build(): KcClassMembers {
        val candidates = ArrayList<Candidate>()
        val statics = c.members.filter(::isStaticMember).toMutableList()
        for (st in c.superTypes) {
            val ct = rel.classType(st) ?: continue
            val sc = ct.symbol as? KcClassSymbol ?: continue
            val sub = rel.typeSubstitutor(ct)
            val inherited = round.members(sc)
            for (m in inherited.callables) {
                if (m.visibility == KaSymbolVisibility.PRIVATE) continue
                candidates += Candidate(m, sub)
            }
            if ((sc.origin == KaSymbolOrigin.JAVA_LIBRARY || sc.origin == KaSymbolOrigin.JAVA_SOURCE) && sc.classKind == KaClassKind.CLASS) {
                for (s in inherited.statics) {
                    if (s !is KaCallableSymbol || s.visibility == KaSymbolVisibility.PRIVATE) continue
                    if (statics.none { it is KaCallableSymbol && matches(Candidate(it, rel.empty), Candidate(s, rel.empty)) }) statics += s
                }
            }
        }

        val java = (c.origin == KaSymbolOrigin.JAVA_SOURCE || c.origin == KaSymbolOrigin.JAVA_LIBRARY) &&
            c.classKind != KaClassKind.ANNOTATION_CLASS
        val all = if (java) javaDeclared(candidates) else null
        val declared = (all ?: c.members).filterIsInstance<KaCallableSymbol>()
            .filter { it !is KaConstructorSymbol && !isStaticMember(it as KaDeclarationSymbol) }

        val overridden = IdentityHashMap<KaCallableSymbol, List<KaCallableSymbol>>()
        val enhanced = IdentityHashMap<KaCallableSymbol, KcEnhancement>()
        val pending = candidates.toMutableList()
        for (d in declared) {
            val dc = Candidate(d, rel.empty)
            val erased = if (java && d is KaNamedFunctionSymbol) candidates.filter { erasedMatch(d, it) } else emptyList()
            val hits = candidates.filter { (matches(dc, it) || erased.any { e -> e === it }) && (!java || returnCompatible(d, it)) }
            if (d is KcFunctionSymbol && d.info.isJava && c.classKind != KaClassKind.ANNOTATION_CLASS) {
                val supers = hits.flatMap(::unwrap).filter { it.symbol is KaFunctionSymbol } + getterOf(d, candidates, java)
                enhancer.enhance(c, d, supers.map { it.symbol to mapped(it.sub, java) }, erasedTypes(d, erased.firstOrNull()))?.let { enhanced[d] = it }
            }
            if (hits.isEmpty()) continue
            overridden[d] = hits.flatMap { declaredOriginals(it.symbol) }.distinct()
            pending.removeAll { p -> hits.any { it === p } }
        }
        // A static JDK method has only a predefined enhancement: Optional.of.
        // A getter folded into a property (see javaDeclared) is still a
        // function of the class, enhanced from the property it implements.
        val listed = IdentityHashMap<KaCallableSymbol, Boolean>().apply { declared.forEach { put(it, true) } }
        for (s in c.members) {
            if (s !is KcFunctionSymbol || !s.info.isJava) continue
            if (isStaticMember(s)) enhancer.enhance(c, s, emptyList(), emptyMap(), staticOnly = true)?.let { enhanced[s] = it }
            else if (java && !listed.containsKey(s)) getterOf(s, candidates, true).takeIf { it.isNotEmpty() }?.let { props ->
                enhancer.enhance(c, s, props.map { it.symbol to it.sub }, emptyMap())?.let { enhanced[s] = it }
            }
        }

        val callables = ArrayList<KaCallableSymbol>(declared)
        if (java) {
            // An inherited Java getter that implements an inherited Kotlin
            // property: FIR folds them into a property of this class.
            for (f in pending.toList()) {
                val fs = f.symbol as? KaNamedFunctionSymbol ?: continue
                if (fs.valueParameters.isNotEmpty() || f !in pending) continue
                val props = pending.filter { isProperty(it.symbol) && getterName(it.symbol as KaPropertySymbol) == fs.name.asString() }
                if (props.isEmpty()) continue
                val setter = pending.firstOrNull { s ->
                    val ss = s.symbol as? KaNamedFunctionSymbol ?: return@firstOrNull false
                    ss.valueParameters.size == 1 && props.any { !(it.symbol as KaPropertySymbol).isVal && setterName(it.symbol as KaPropertySymbol) == ss.name.asString() }
                }
                val key = c.info.key + "/~" + callables.size
                val synth = KcSyntheticPropertySymbol(c, (props[0].symbol as KaPropertySymbol).name, fs, setter?.symbol as KaNamedFunctionSymbol?, f.sub, key)
                overridden[synth] = props.flatMap { declaredOriginals(it.symbol) }.distinct()
                pending.removeAll { p -> p === f || p === setter || props.any { it === p } }
                callables += synth
            }
        }

        val groups = ArrayList<MutableList<Candidate>>()
        for (p in pending) {
            groups.firstOrNull { matches(it[0], p) }?.add(p) ?: groups.add(mutableListOf(p))
        }

        for (g in groups) {
            val members = mostSpecific(g)
            val key = c.info.key + "/~" + callables.size
            callables += if (members.size == 1) {
                val m = members[0]
                if (unchanged(m)) m.symbol else fake(m, KaSymbolOrigin.SUBSTITUTION_OVERRIDE, emptyList(), key)
            } else {
                // FIR's most specific member: one whose return type is a
                // subtype of all the others', a non-abstract one first.
                val ret = members.associateWith { it.sub.substitute(it.symbol.returnType) }
                val specific = members.filter { m ->
                    members.all { o -> o === m || round.relations.isSubtype(ret.getValue(m), ret.getValue(o), true) }
                }.ifEmpty { members }
                val chosen = specific.firstOrNull { it.symbol.modality != KaSymbolModality.ABSTRACT } ?: specific[0]
                fake(chosen, KaSymbolOrigin.INTERSECTION_OVERRIDE, members.map { it.symbol }, key, members.map { it.sub })
            }
        }
        return KcClassMembers(callables, overridden, statics, all, enhanced)
    }

    /**
     * The types FIR gives the parameters of Java method [d] that Java
     * erases, by index: those of the built-in member [erased] it implements
     * (the first, if it implements it through several supertypes).
     */
    private fun erasedTypes(d: KaCallableSymbol, erased: Candidate?): Map<Int, KaType> {
        if (erased == null || d !is KaNamedFunctionSymbol) return emptyMap()
        val ps = (erased.symbol as KaFunctionSymbol).valueParameters
        return erasedParams[d.name.asString()].orEmpty().filter { it < ps.size }.associateWith { erased.sub.substitute(ps[it].returnType) }
    }

    /** The Kotlin properties among [candidates] that Java method [d] of [c] is the getter of, if [java]: MutableMap.keys for AbstractMap.keySet(). */
    private fun getterOf(d: KaNamedFunctionSymbol, candidates: List<Candidate>, java: Boolean): List<Candidate> =
        if (!java || d.valueParameters.isNotEmpty() || d.typeParameters.isNotEmpty()) emptyList()
        else candidates.filter { isProperty(it.symbol) && getterName(it.symbol as KaPropertySymbol) == d.name.asString() }

    /**
     * Members [ms] but those another of them overrides, as FIR's
     * nonSubsumed: MutableList's spliterator() stands for List's, not also
     * the Iterable.spliterator() that overrides, which MutableIterable has
     * with an inflexible argument.
     */
    private fun nonSubsumed(ms: List<Candidate>): List<Candidate> {
        if (ms.size < 2) return ms
        val bases = IdentityHashMap<KaCallableSymbol, Boolean>()
        for (m in ms) round.allOverridden(fakeOverrideOriginal(m.symbol)).forEach { bases[it] = true }
        return ms.filter { !bases.containsKey(fakeOverrideOriginal(it.symbol)) }
    }

    /**
     * Substitutor [sub] of a supertype's arguments, for enhancing a member
     * of [c]: unless [java], so in a Kotlin class, a JDK method's
     * (`jvmMappedMembers`), with [c]'s type parameters flexible after
     * it. FIR has these methods from
     * the JDK's classes, whose supertypes' arguments are flexible, so
     * List.spliterator() returns `Spliterator<E!>`, not Iterable's
     * `Spliterator<T>` with List's `E`.
     */
    private fun mapped(sub: KaSubstitutor, java: Boolean): KaSubstitutor {
        if (java || c.typeParameters.isEmpty()) return sub
        return KcChainedSubstitutor(sub, rel.substitutor(c.typeParameters.associateWith {
            enhancer.flexibleOf(KcTypeParameterType(round, it.name, it, false))
        }))
    }

    /**
     * [h], or the members an intersection override [h] stands for (but
     * those another of them overrides), each with its substitutor in [c].
     * A class's direct supertypes' members all count: ArrayList.subList()
     * overrides AbstractList's and MutableList's.
     */
    private fun unwrap(h: Candidate): List<Candidate> {
        val s = h.symbol as? KcFakeFunctionSymbol ?: return listOf(h)
        if (s.intersected.isEmpty()) return listOf(h)
        return nonSubsumed(s.intersected.zip(s.intersectedSubs).flatMap { (m, sub) -> unwrap(Candidate(m, KcChainedSubstitutor(sub, h.sub))) })
    }

    /**
     * Whether Java method [d] implements built-in collection member [y]
     * whose parameter Java erases: `contains(Object)` implements
     * `Collection.contains(element: E)`, `containsAll(Collection<?>)`
     * `Collection.containsAll(elements: Collection<E>)` (FIR's
     * SpecialGenericSignatures). FIR gives [d] those parameters' types.
     */
    private fun erasedMatch(d: KaNamedFunctionSymbol, y: Candidate): Boolean {
        val erased = erasedParams[d.name.asString()] ?: return false
        val b = y.symbol as? KaNamedFunctionSymbol ?: return false
        if (b.name != d.name || b.valueParameters.size != d.valueParameters.size || d.typeParameters.isNotEmpty() ||
            b.typeParameters.isNotEmpty() || b.receiverParameter != null
        ) return false
        if (declaredOriginals(b).none { it.callableId?.packageName?.asString() == "kotlin.collections" }) return false
        return d.valueParameters.indices.all { i ->
            val dt = d.valueParameters[i].returnType
            if (i in erased) isErased(enhancer.flexibleOf(dt)) else sameType(enhancer.flexibleOf(dt), rel.empty, b.valueParameters[i].returnType, y.sub)
        }
    }

    /** Whether [t] is Java's `Object` or `Collection<?>`, as an erased built-in parameter is. */
    private fun isErased(t: KaType): Boolean {
        val lo = (t as? KcFlexibleType)?.lowerBound as? KaClassType ?: return false
        return when (lo.classId.asString()) {
            "kotlin/Any" -> true
            "kotlin/collections/MutableCollection" -> lo.typeArguments.singleOrNull() is KaStarTypeProjection
            else -> false
        }
    }

    /**
     * The members of a group of matching candidates, one per declaration
     * they come from. As in FIR, one that another overrides stays: `class
     * B : A(), I`, where `A.f` overrides `I.f`, has an intersection
     * override of both.
     */
    private fun mostSpecific(g: List<Candidate>): List<Candidate> {
        val distinct = ArrayList<Candidate>()
        val seen = IdentityHashMap<KaCallableSymbol, Boolean>()
        for (m in g) if (seen.put(fakeOverrideOriginal(m.symbol), true) == null) distinct += m
        return distinct
    }

    /**
     * The declarations of Java class [c] as FIR has them: Object's methods
     * dropped from an interface; a getter (and setter) implementing an
     * inherited Kotlin property, from [candidates], folded into a
     * [KcSyntheticPropertySymbol]; and a method implementing a built-in
     * member with another JVM name renamed ([KcRenamedFunctionSymbol]).
     */
    private fun javaDeclared(candidates: List<Candidate>): List<KaDeclarationSymbol> {
        val props = candidates.filter { isProperty(it.symbol) }
        val interfaceKind = c.classKind == KaClassKind.INTERFACE
        val funs = c.members.filterIsInstance<KaNamedFunctionSymbol>().filter { !it.isStatic }
        val setters = IdentityHashMap<KaNamedFunctionSymbol, Boolean>()
        // Each folded getter: the property's name and the setter.
        val folded = IdentityHashMap<KaNamedFunctionSymbol, Pair<Name, KaNamedFunctionSymbol?>>()
        for (d in funs) {
            if (d.valueParameters.isNotEmpty() || d.typeParameters.isNotEmpty()) continue
            val ps = props.firstOrNull { getterName(it.symbol as KaPropertySymbol) == d.name.asString() }?.symbol as KaPropertySymbol? ?: continue
            val setter = if (ps.isVal) null else funs.firstOrNull {
                it.valueParameters.size == 1 && it.typeParameters.isEmpty() && it.name.asString() == setterName(ps)
            }
            // FIR hides the Java methods only if their name gives back the
            // property's: getFoo implements val Foo, but stays a function too.
            if (setter != null && hides(d, ps)) setters[setter] = true
            folded[d] = ps.name to setter
        }
        // Keys of folded and renamed members index the declarations.
        val out = ArrayList<KaDeclarationSymbol>()
        for (m in c.members) {
            val key = c.info.key + "/~d" + out.size
            if (m !is KaNamedFunctionSymbol || m.isStatic) {
                out += m
                continue
            }
            if (interfaceKind && isObjectMethod(m) || setters.containsKey(m)) continue
            val f = folded[m]
            val renamed = renames[m.name.asString()]
            if (f != null && !hides(m, f.first)) {
                out += m
                out += KcSyntheticPropertySymbol(c, f.first, m, f.second, rel.empty, c.info.key + "/~d" + out.size)
                continue
            }
            out += when {
                f != null -> KcSyntheticPropertySymbol(c, f.first, m, f.second, rel.empty, key)
                renamed != null && candidates.any { sameParams(m, it, renamed) } ->
                    KcRenamedFunctionSymbol(c, m, Name.identifier(renamed), key)
                else -> m
            }
        }
        return out
    }

    private fun hides(getter: KaNamedFunctionSymbol, p: KaPropertySymbol): Boolean =
        isBuiltin(p) && builtinGetters.containsKey(p.name.asString()) || hides(getter, p.name)

    /** Whether getter's name gives back property name [p], as FIR's propertyNameByGetMethodName. */
    private fun hides(getter: KaNamedFunctionSymbol, p: Name): Boolean {
        val n = getter.name.asString()
        if (n.startsWith("is") && n.length > 2 && !n[2].isLowerCase()) return n == p.asString()
        if (!n.startsWith("get") || n.length == 3 || n[3].isLowerCase()) return builtinGetters[p.asString()] == n
        return decapitalizeSmart(n.substring(3)) == p.asString()
    }

    /** Whether [s] is a property a Java method can implement: a Kotlin member property, or a synthetic one. */
    private fun isProperty(s: KaCallableSymbol): Boolean = when (s) {
        is KcSyntheticPropertySymbol -> true
        is KaKotlinPropertySymbol -> s.receiverParameter == null && s.origin != KaSymbolOrigin.JAVA_SOURCE &&
            s.origin != KaSymbolOrigin.JAVA_LIBRARY && s.origin != KaSymbolOrigin.JAVA_SYNTHETIC_PROPERTY
        else -> false
    }

    /** The JVM name of [p]'s getter: `getX`, `isX`, or a built-in's own (`size`, `keySet`, `entrySet`, …). */
    private fun getterName(p: KaPropertySymbol): String {
        if (p is KcSyntheticPropertySymbol) return p.javaGetterSymbol.name.asString()
        val n = p.name.asString()
        if (isBuiltin(p)) builtinGetters[n]?.let { return it }
        val isName = n.startsWith("is") && n.length > 2 && !n[2].isLowerCase()
        return if (isName) n else "get" + n.replaceFirstChar { it.uppercaseChar() }
    }

    /** The JVM name of [p]'s setter. */
    private fun setterName(p: KaPropertySymbol): String {
        if (p is KcSyntheticPropertySymbol) return p.javaSetterSymbol?.name?.asString() ?: ""
        val n = p.name.asString()
        val isName = n.startsWith("is") && n.length > 2 && !n[2].isLowerCase()
        return "set" + if (isName) n.substring(2) else n.replaceFirstChar { it.uppercaseChar() }
    }

    /** Whether [p] is declared by a built-in class (`kotlin.collections.*`, `kotlin.CharSequence`). */
    private fun isBuiltin(p: KaCallableSymbol): Boolean {
        val id = fakeOverrideOriginal(p).callableId?.classId ?: return false
        return id.packageFqName.asString() == "kotlin.collections" || id.asString() == "kotlin/CharSequence"
    }

    /** Whether [m] is `equals(Object)`, `hashCode()` or `toString()`. */
    private fun isObjectMethod(m: KaNamedFunctionSymbol): Boolean = when (m.name.asString()) {
        "hashCode", "toString" -> m.valueParameters.isEmpty()
        "equals" -> m.valueParameters.size == 1 &&
            (rel.classType(m.valueParameters[0].returnType)?.classId?.asString() == "kotlin/Any")
        else -> false
    }

    /** Whether candidate [c] is a built-in function named [name] with [m]'s parameter types. */
    private fun sameParams(m: KaNamedFunctionSymbol, c: Candidate, name: String): Boolean {
        val f = c.symbol as? KaNamedFunctionSymbol ?: return false
        if (f.name.asString() != name || f.valueParameters.size != m.valueParameters.size || !isBuiltinMember(f)) return false
        return m.valueParameters.indices.all { sameType(m.valueParameters[it].returnType, rel.empty, f.valueParameters[it].returnType, c.sub) }
    }

    /** Whether [f] is declared by a built-in class. */
    private fun isBuiltinMember(f: KaCallableSymbol): Boolean = f is KcRenamedFunctionSymbol ||
        fakeOverrideOriginal(f).callableId?.classId?.packageFqName?.asString()?.let { it == "kotlin" || it == "kotlin.collections" } == true

    /** Whether [m]'s substitutor changes none of its types. */
    private fun unchanged(m: Candidate): Boolean {
        val s = m.symbol
        if (m.sub.substituteOrNull(s.returnType) != null) return false
        if (s.receiverType?.let(m.sub::substituteOrNull) != null) return false
        return s !is KaFunctionSymbol || s.valueParameters.none { m.sub.substituteOrNull(it.returnType) != null }
    }

    private fun fake(
        m: Candidate, origin: KaSymbolOrigin, intersected: List<KaCallableSymbol>, key: String,
        subs: List<KaSubstitutor> = emptyList(),
    ): KaCallableSymbol =
        when (val s = m.symbol) {
            is KaNamedFunctionSymbol -> KcFakeFunctionSymbol(c, s, m.sub, origin, intersected, key, subs)
            is KaKotlinPropertySymbol -> KcFakePropertySymbol(c, s, m.sub, origin, intersected, key)
            is KcSyntheticPropertySymbol -> KcFakeSyntheticPropertySymbol(c, s, m.sub, origin, intersected, key)
            else -> s // Java fields keep their declared types.
        }

    /**
     * Whether candidates [x] and [y] have the same signature: functions of
     * the same name, receiver, parameter and type parameter counts whose
     * receiver and parameter types are equal once substituted, [y]'s type
     * parameters taken for [x]'s; properties of the same name and receiver;
     * Java fields of the same name.
     */
    private fun matches(x: Candidate, y: Candidate): Boolean {
        val a = x.symbol
        val b = y.symbol
        return when {
            a is KaNamedFunctionSymbol && b is KaNamedFunctionSymbol -> {
                if (a.name != b.name || a.valueParameters.size != b.valueParameters.size ||
                    a.typeParameters.size != b.typeParameters.size ||
                    (a.receiverParameter == null) != (b.receiverParameter == null)
                ) return false
                val sb = positional(y.sub, b.typeParameters, a.typeParameters)
                val la = isJava(a)
                val lb = isJava(b)
                sameType(a.receiverType, x.sub, b.receiverType, sb) &&
                    a.valueParameters.indices.all {
                        sameType(loose(paramType(a, it, la && lb), la), x.sub, loose(paramType(b, it, la && lb), lb), sb)
                    }
            }
            a is KaPropertySymbol && b is KaPropertySymbol ->
                a.name == b.name && (a.receiverParameter == null) == (b.receiverParameter == null) &&
                    sameType(a.receiverType, x.sub, b.receiverType, positional(y.sub, b.typeParameters, a.typeParameters))
            a is KaJavaFieldSymbol && b is KaJavaFieldSymbol -> a.name == b.name
            else -> false
        }
    }

    /** A Java method overrides only with a covariant return type; FIR drops the rest. */
    private fun returnCompatible(d: KaCallableSymbol, m: Candidate): Boolean {
        val a = d as? KaNamedFunctionSymbol ?: return true
        val b = m.symbol as? KaNamedFunctionSymbol ?: return true
        val sb = positional(m.sub, b.typeParameters, a.typeParameters)
        return rel.isSubtype(loose(a.returnType, isJava(a)), sb.substitute(loose(b.returnType, isJava(b))), lenient = true)
    }

    /**
     * Function [f]'s parameter [i]'s type: as declared, if [java], as FIR
     * matches a Java method's Java overrides before enhancing them.
     * `HashMap.get(Object)` overrides `AbstractMap.get(Object)`, whose
     * enhanced type is `K!`; AbstractCollection's `contains(Object)`, as
     * `contains(E)`, matches Kotlin's `Set.contains(E)`.
     */
    private fun paramType(f: KaFunctionSymbol, i: Int, java: Boolean): KaType = when {
        !java -> f.valueParameters[i].returnType
        f is KcFakeFunctionSymbol -> f.sub.substitute(paramType(f.original, i, true))
        else -> (f.valueParameters[i] as? KcValueParameterSymbol)?.declaredType ?: f.valueParameters[i].returnType
    }

    private fun isJava(s: KaCallableSymbol) = fakeOverrideOriginal(s).let { it is KcFunctionSymbol && it.info.isJava }

    /**
     * Type [t] of a Java method, if [java], as FIR matches overrides: the
     * flexible type its nullability annotation, or an enhancement, made it
     * not, at every position (`KcEnhancer.deepFlexibleOf`).
     * `@NotNull String f(@NotNull String x)` overrides `f(x: String?): String`.
     */
    private fun loose(t: KaType, java: Boolean) = if (java) enhancer.deepFlexibleOf(t) else t

    /** [sub] followed by the substitution of type parameters [from] by [to]'s types, position by position. */
    private fun positional(sub: KaSubstitutor, from: List<KaTypeParameterSymbol>, to: List<KaTypeParameterSymbol>): KaSubstitutor {
        if (from.isEmpty() || from.size != to.size) return sub
        val map = from.zip(to).associate { (f, t) -> f to (KcTypeParameterType(round, t.name, t, false) as KaType) }
        return KcChainedSubstitutor(sub, rel.substitutor(map))
    }

    private fun sameType(a: KaType?, sa: KaSubstitutor, b: KaType?, sb: KaSubstitutor): Boolean {
        if (a == null || b == null) return a == null && b == null
        return rel.equal(sa.substitute(a), sb.substitute(b), lenient = true)
    }
}

/**
 * The built-in collection members whose parameters (by index) Java erases
 * to `Object` or `Collection<?>`: both of MutableMap's `remove(key, value)`.
 */
private val erasedParams = mapOf(
    "contains" to setOf(0), "containsAll" to setOf(0), "remove" to setOf(0, 1), "removeAll" to setOf(0),
    "retainAll" to setOf(0), "indexOf" to setOf(0), "lastIndexOf" to setOf(0), "containsKey" to setOf(0),
    "containsValue" to setOf(0), "get" to setOf(0), "getOrDefault" to setOf(0),
)

/** The JVM getter names of built-in properties that don't follow `getX`. */
private val builtinGetters = mapOf("size" to "size", "keys" to "keySet", "values" to "values", "entries" to "entrySet", "length" to "length")

/** The Kotlin names of Java methods that implement built-in members with other JVM names. */
private val renames = mapOf(
    "remove" to "removeAt", "charAt" to "get", "byteValue" to "toByte", "shortValue" to "toShort",
    "intValue" to "toInt", "longValue" to "toLong", "floatValue" to "toFloat", "doubleValue" to "toDouble",
)

/** A function a class inherits under a new signature: a substitution or intersection override. */
internal class KcFakeFunctionSymbol(
    override val owner: KcClassSymbol,
    override val original: KaNamedFunctionSymbol,
    val sub: KaSubstitutor,
    override val origin: KaSymbolOrigin,
    override val intersected: List<KaCallableSymbol>,
    private val key: String,
    /** The substitutors of [intersected] in [owner]'s supertypes. */
    val intersectedSubs: List<KaSubstitutor> = emptyList(),
) : StubKaNamedFunctionSymbol(), KcFakeOverride {
    override val token get() = owner.token
    override val psi get() = original.psi
    override val annotations: KaAnnotationList get() = original.annotations
    override val name get() = original.name
    override val callableId get() = owner.classId?.let { CallableId(it, name) }
    override val visibility get() = original.visibility
    override val compilerVisibility get() = compilerVisibilityOf(original.visibility)
    override val location get() = KaSymbolLocation.CLASS
    override val modality get() = original.modality
    override val returnType: KaType by lazy { sub.substitute(original.returnType) }
    override val receiverParameter: KaReceiverParameterSymbol? by lazy {
        original.receiverParameter?.let { KcFakeReceiverParameterSymbol(this, it, sub.substitute(it.returnType), "$key/rcv") }
    }
    override val isSuspend get() = original.isSuspend
    override val isInline get() = original.isInline
    override val isOperator get() = original.isOperator
    override val isInfix get() = original.isInfix
    override val isTailRec get() = original.isTailRec
    override val isExternal get() = original.isExternal
    override val isOverride get() = true
    override val isExpect get() = original.isExpect
    override val isActual get() = original.isActual
    override val isExtension get() = original.isExtension
    override val isStatic get() = false
    override val isBuiltinFunctionInvoke get() = false
    override val hasStableParameterNames get() = original.hasStableParameterNames
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val typeParameters get() = original.typeParameters
    override val valueParameters: List<KaValueParameterSymbol> by lazy {
        original.valueParameters.mapIndexed { i, p -> KcFakeValueParameterSymbol(this, p, sub.substitute(p.returnType), "$key/($i") }
    }
    override fun createPointer(): KaSymbolPointer<KaNamedFunctionSymbol> = KcPointer(key)
    override fun toString() = "$origin ${owner.classId}.$name"
}

/** A property a class inherits under a new signature. */
internal class KcFakePropertySymbol(
    override val owner: KcClassSymbol,
    override val original: KaKotlinPropertySymbol,
    val sub: KaSubstitutor,
    override val origin: KaSymbolOrigin,
    override val intersected: List<KaCallableSymbol>,
    private val key: String,
) : StubKaKotlinPropertySymbol(), KcFakeOverride {
    override val token get() = owner.token
    override val psi get() = original.psi
    override val annotations: KaAnnotationList get() = original.annotations
    override val name get() = original.name
    override val callableId get() = owner.classId?.let { CallableId(it, name) }
    override val visibility get() = original.visibility
    override val compilerVisibility get() = compilerVisibilityOf(original.visibility)
    override val location get() = KaSymbolLocation.CLASS
    override val modality get() = original.modality
    override val isVal get() = original.isVal
    override val isFromPrimaryConstructor get() = original.isFromPrimaryConstructor
    override val isConst get() = original.isConst
    override val isLateInit get() = original.isLateInit
    override val isOverride get() = true
    override val isExternal get() = original.isExternal
    override val isExpect get() = original.isExpect
    override val isActual get() = original.isActual
    override val isDelegated get() = original.isDelegated
    // FIR's substitution and intersection overrides have no backing field.
    override val hasBackingField get() = false
    override val isExtension get() = original.isExtension
    override val isStatic get() = false
    override val returnType: KaType by lazy { sub.substitute(original.returnType) }
    override val receiverParameter: KaReceiverParameterSymbol? by lazy {
        original.receiverParameter?.let { KcFakeReceiverParameterSymbol(this, it, sub.substitute(it.returnType), "$key/rcv") }
    }
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val typeParameters get() = original.typeParameters
    override val hasGetter get() = original.hasGetter
    override val hasSetter get() = original.hasSetter
    override val backingFieldSymbol: KaBackingFieldSymbol? get() = null
    override val getter: KaPropertyGetterSymbol? by lazy { original.getter?.let { KcFakeGetterSymbol(this, it, "$key/get") } }
    override val setter: KaPropertySetterSymbol? by lazy { original.setter?.let { KcFakeSetterSymbol(this, it, "$key/set") } }
    override fun createPointer(): KaSymbolPointer<KaKotlinPropertySymbol> = KcPointer(key)
    override fun toString() = "$origin ${owner.classId}.$name"
}

internal class KcFakeGetterSymbol(
    val property: KcFakePropertySymbol,
    private val original: KaPropertyGetterSymbol,
    private val key: String,
) : StubKaPropertyGetterSymbol() {
    override val token get() = property.token
    override val psi get() = original.psi
    override val origin get() = property.origin
    override val annotations: KaAnnotationList get() = original.annotations
    override val callableId: CallableId? get() = null
    override val visibility get() = original.visibility
    override val compilerVisibility get() = compilerVisibilityOf(original.visibility)
    override val modality get() = property.modality
    override val returnType get() = property.returnType
    override val isDefault get() = original.isDefault
    override val isNotDefault get() = original.isNotDefault
    override val hasBody get() = original.hasBody
    override val isInline get() = original.isInline
    override val isExternal get() = original.isExternal
    override val isOverride get() = true
    override val receiverParameter get() = property.receiverParameter
    override fun createPointer(): KaSymbolPointer<KaPropertyGetterSymbol> = KcPointer(key)
}

internal class KcFakeSetterSymbol(
    val property: KcFakePropertySymbol,
    private val original: KaPropertySetterSymbol,
    private val key: String,
) : StubKaPropertySetterSymbol() {
    override val token get() = property.token
    override val psi get() = original.psi
    override val origin get() = property.origin
    override val annotations: KaAnnotationList get() = original.annotations
    override val callableId: CallableId? get() = null
    override val visibility get() = original.visibility
    override val compilerVisibility get() = compilerVisibilityOf(original.visibility)
    override val modality get() = property.modality
    override val returnType get() = original.returnType
    override val isDefault get() = original.isDefault
    override val isNotDefault get() = original.isNotDefault
    override val hasBody get() = original.hasBody
    override val hasStableParameterNames get() = original.hasStableParameterNames
    override val isInline get() = original.isInline
    override val isExternal get() = original.isExternal
    override val isOverride get() = true
    override val receiverParameter get() = property.receiverParameter
    override val parameter: KaValueParameterSymbol by lazy {
        KcFakeValueParameterSymbol(this, original.parameter, property.returnType, "$key/value")
    }
    override fun createPointer(): KaSymbolPointer<KaPropertySetterSymbol> = KcPointer(key)
}

/** A value parameter of a fake override, [owner], of type [returnType]. */
internal class KcFakeValueParameterSymbol(
    val owner: KaCallableSymbol,
    private val original: KaValueParameterSymbol,
    override val returnType: KaType,
    private val key: String,
) : StubKaValueParameterSymbol() {
    override val token get() = owner.token
    override val psi get() = original.psi
    override val origin get() = owner.origin
    override val annotations: KaAnnotationList get() = original.annotations
    override val name get() = original.name
    override val visibility get() = KaSymbolVisibility.LOCAL
    override val compilerVisibility get() = compilerVisibilityOf(original.visibility)
    override val isVararg get() = original.isVararg
    override val isCrossinline get() = original.isCrossinline
    override val isNoinline get() = original.isNoinline
    override val hasDeclaredDefaultValue get() = original.hasDeclaredDefaultValue
    override val hasDefaultValue get() = original.hasDefaultValue
    override val isImplicitLambdaParameter get() = false
    override fun createPointer(): KaSymbolPointer<KaValueParameterSymbol> = KcPointer(key)
}

internal class KcFakeReceiverParameterSymbol(
    override val owningCallableSymbol: KaCallableSymbol,
    private val original: KaReceiverParameterSymbol,
    override val returnType: KaType,
    private val key: String,
) : StubKaReceiverParameterSymbol() {
    override val token get() = owningCallableSymbol.token
    override val psi get() = original.psi
    override val origin get() = owningCallableSymbol.origin
    override val annotations: KaAnnotationList get() = original.annotations
    override fun createPointer(): KaSymbolPointer<KaReceiverParameterSymbol> = KcPointer(key)
}

/**
 * A Java class's getter, and setter if any, that implement a Kotlin
 * property it inherits, folded into a property of [owner] as FIR does:
 * the functions leave the class's scopes and this property overrides the
 * Kotlin one.
 */
internal open class KcSyntheticPropertySymbol(
    val owner: KcClassSymbol,
    override val name: Name,
    override val javaGetterSymbol: KaNamedFunctionSymbol,
    override val javaSetterSymbol: KaNamedFunctionSymbol?,
    val subst: KaSubstitutor,
    private val key: String,
) : StubKaSyntheticJavaPropertySymbol() {
    override val token get() = owner.token
    override val psi get() = javaGetterSymbol.psi
    override val annotations: KaAnnotationList get() = javaGetterSymbol.annotations
    override val callableId get() = owner.classId?.let { CallableId(it, name) }
    override val visibility get() = javaGetterSymbol.visibility
    override val compilerVisibility get() = compilerVisibilityOf(visibility)
    override val modality get() = javaGetterSymbol.modality
    override val isVal get() = javaSetterSymbol == null
    override val isOverride get() = true
    override val isExtension get() = false
    override val isStatic get() = false
    override val isExpect get() = false
    override val isActual get() = false
    override val isExternal get() = false
    override val hasSetter get() = javaSetterSymbol != null
    override val receiverParameter: KaReceiverParameterSymbol? get() = null
    override val typeParameters: List<KaTypeParameterSymbol> get() = emptyList()
    override val returnType: KaType by lazy { subst.substitute(javaGetterSymbol.returnType) }
    override val getter: KaPropertyGetterSymbol by lazy { KcSyntheticGetterSymbol(this, "$key/get") }
    override val setter: KaPropertySetterSymbol? by lazy { javaSetterSymbol?.let { KcSyntheticSetterSymbol(this, it, "$key/set") } }
    override fun createPointer(): KaSymbolPointer<KaSyntheticJavaPropertySymbol> = KcPointer(key)
    override fun toString() = "$origin ${owner.classId}.$name"
}

/** A synthetic property a class inherits under a new signature. */
internal class KcFakeSyntheticPropertySymbol(
    owner: KcClassSymbol,
    override val original: KcSyntheticPropertySymbol,
    sub: KaSubstitutor,
    override val origin: KaSymbolOrigin,
    override val intersected: List<KaCallableSymbol>,
    key: String,
) : KcSyntheticPropertySymbol(owner, original.name, original.javaGetterSymbol, original.javaSetterSymbol, KcChainedSubstitutor(original.subst, sub), key),
    KcFakeOverride

/** The getter of synthetic property [property]: its Java getter method. */
internal class KcSyntheticGetterSymbol(
    val property: KcSyntheticPropertySymbol,
    private val key: String,
) : StubKaPropertyGetterSymbol() {
    private val java get() = property.javaGetterSymbol
    override val token get() = property.token
    override val psi get() = java.psi
    override val origin get() = KaSymbolOrigin.JAVA_SYNTHETIC_PROPERTY
    override val annotations: KaAnnotationList get() = java.annotations
    override val callableId: CallableId? get() = property.owner.classId?.let { CallableId(it, java.name) }
    override val visibility get() = java.visibility
    override val compilerVisibility get() = compilerVisibilityOf(java.visibility)
    override val modality get() = java.modality
    override val returnType get() = property.returnType
    override val isDefault get() = false
    override val isNotDefault get() = true
    override val hasBody get() = java.modality != KaSymbolModality.ABSTRACT
    override val isInline get() = false
    override val isExternal get() = false
    override val isOverride get() = true
    override val receiverParameter: KaReceiverParameterSymbol? get() = null
    override fun createPointer(): KaSymbolPointer<KaPropertyGetterSymbol> = KcPointer(key)
}

/** The setter of synthetic property [property]: Java setter method [java]. */
internal class KcSyntheticSetterSymbol(
    val property: KcSyntheticPropertySymbol,
    private val java: KaNamedFunctionSymbol,
    private val key: String,
) : StubKaPropertySetterSymbol() {
    override val token get() = property.token
    override val psi get() = java.psi
    override val origin get() = KaSymbolOrigin.JAVA_SYNTHETIC_PROPERTY
    override val annotations: KaAnnotationList get() = java.annotations
    override val callableId: CallableId? get() = property.owner.classId?.let { CallableId(it, java.name) }
    override val visibility get() = java.visibility
    override val compilerVisibility get() = compilerVisibilityOf(java.visibility)
    override val modality get() = java.modality
    override val returnType get() = java.returnType
    override val isDefault get() = false
    override val isNotDefault get() = true
    override val hasBody get() = java.modality != KaSymbolModality.ABSTRACT
    override val hasStableParameterNames get() = false
    override val isInline get() = false
    override val isExternal get() = false
    override val isOverride get() = true
    override val receiverParameter: KaReceiverParameterSymbol? get() = null
    override val parameter: KaValueParameterSymbol get() = java.valueParameters[0]
    override fun createPointer(): KaSymbolPointer<KaPropertySetterSymbol> = KcPointer(key)
}

/**
 * A Java method FIR gives a Kotlin name, [name], because it implements a
 * built-in member that has a different JVM name: `remove(int)` is
 * `removeAt`, `charAt` is `get`, `intValue` is `toInt`, ….
 */
internal class KcRenamedFunctionSymbol(
    val owner: KcClassSymbol,
    val java: KaNamedFunctionSymbol,
    override val name: Name,
    private val key: String,
) : StubKaNamedFunctionSymbol() {
    override val token get() = owner.token
    override val psi get() = java.psi
    override val origin get() = java.origin
    override val annotations: KaAnnotationList get() = java.annotations
    override val callableId get() = owner.classId?.let { CallableId(it, name) }
    override val visibility get() = java.visibility
    override val compilerVisibility get() = compilerVisibilityOf(java.visibility)
    override val location get() = KaSymbolLocation.CLASS
    override val modality get() = java.modality
    override val returnType get() = java.returnType
    override val receiverParameter: KaReceiverParameterSymbol? get() = null
    override val isSuspend get() = false
    override val isInline get() = false
    override val isOperator get() = java.isOperator
    override val isInfix get() = false
    override val isTailRec get() = false
    override val isExternal get() = java.isExternal
    override val isOverride get() = true
    override val isExpect get() = false
    override val isActual get() = false
    override val isExtension get() = false
    override val isStatic get() = false
    override val isBuiltinFunctionInvoke get() = false
    override val hasStableParameterNames get() = false
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val typeParameters get() = java.typeParameters
    override val valueParameters get() = java.valueParameters
    override fun createPointer(): KaSymbolPointer<KaNamedFunctionSymbol> = KcPointer(key)
    override fun toString() = "$origin ${owner.classId}.$name"
}

/** Kotlin's decapitalizeSmart(asciiOnly = true): URLs → urLs, URL → url, Foo → foo. */
internal fun decapitalizeSmart(s: String): String {
    fun up(i: Int) = s[i] in 'A'..'Z'
    if (s.isEmpty() || !up(0)) return s
    if (s.length == 1 || !up(1)) return s[0].lowercaseChar() + s.substring(1)
    val second = (0 until s.length).firstOrNull { !up(it) }?.let { it - 1 } ?: return s.lowercase()
    return s.substring(0, second).lowercase() + s.substring(second)
}
