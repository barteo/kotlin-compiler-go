@file:Suppress("DEPRECATION", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.aa

import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.base.KaContextReceiver
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.pointers.KaSymbolPointer
import org.jetbrains.kotlin.analysis.api.types.KaSubstitutor
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.descriptors.Visibility
import org.jetbrains.kotlin.descriptors.java.JavaVisibilities
import org.jetbrains.kotlin.name.CallableId
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

    /** A member of a supertype and the substitutor of that supertype's arguments. */
    private class Candidate(val symbol: KaCallableSymbol, val sub: KaSubstitutor)

    fun build(): KcClassMembers {
        var declared = c.members.filterIsInstance<KaCallableSymbol>()
            .filter { it !is KaConstructorSymbol && !isStaticMember(it as KaDeclarationSymbol) }
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

        val overridden = IdentityHashMap<KaCallableSymbol, List<KaCallableSymbol>>()
        val pending = candidates.toMutableList()
        if (c.origin == KaSymbolOrigin.JAVA_SOURCE || c.origin == KaSymbolOrigin.JAVA_LIBRARY) {
            // A Java method that implements an inherited Kotlin property's
            // accessor isn't a function of the class: FIR folds it into
            // the property, which stays a member.
            declared = declared.filterNot { d -> candidates.any { isAccessorOf(d, it.symbol) } }
        }
        for (d in declared) {
            val dc = Candidate(d, rel.empty)
            val hits = candidates.filter { matches(dc, it) }
            if (hits.isEmpty()) continue
            overridden[d] = hits.flatMap { declaredOriginals(it.symbol) }.distinct()
            pending.removeAll { p -> hits.any { it === p } }
        }

        val groups = ArrayList<MutableList<Candidate>>()
        for (p in pending) {
            groups.firstOrNull { matches(it[0], p) }?.add(p) ?: groups.add(mutableListOf(p))
        }

        val callables = ArrayList<KaCallableSymbol>(declared)
        for (g in groups) {
            val members = mostSpecific(g)
            val key = c.info.key + "/~" + callables.size
            callables += if (members.size == 1) {
                val m = members[0]
                if (unchanged(m)) m.symbol else fake(m, KaSymbolOrigin.SUBSTITUTION_OVERRIDE, emptyList(), key)
            } else {
                val chosen = members.firstOrNull { it.symbol.modality != KaSymbolModality.ABSTRACT } ?: members[0]
                fake(chosen, KaSymbolOrigin.INTERSECTION_OVERRIDE, members.map { it.symbol }, key)
            }
        }
        return KcClassMembers(callables, overridden, statics)
    }

    /**
     * The members of a group of matching candidates that no other one
     * overrides, one per declaration they come from.
     */
    private fun mostSpecific(g: List<Candidate>): List<Candidate> {
        val distinct = ArrayList<Candidate>()
        val seen = IdentityHashMap<KaCallableSymbol, Boolean>()
        for (m in g) if (seen.put(fakeOverrideOriginal(m.symbol), true) == null) distinct += m
        if (distinct.size == 1) return distinct
        return distinct.filter { y ->
            val ys = declaredOriginals(y.symbol)
            distinct.none { x -> x !== y && round.allOverridden(x.symbol).any { o -> ys.any { it === o } } }
        }.ifEmpty { distinct }
    }

    /**
     * Whether Java method [d] is the JVM getter or setter of Kotlin
     * property [p]: `getX()`/`isX()` or `setX(v)` of a var.
     */
    private fun isAccessorOf(d: KaCallableSymbol, p: KaCallableSymbol): Boolean {
        if (d !is KaNamedFunctionSymbol || p !is KaKotlinPropertySymbol || p.receiverParameter != null) return false
        if (p.origin == KaSymbolOrigin.JAVA_SOURCE || p.origin == KaSymbolOrigin.JAVA_LIBRARY || p.origin == KaSymbolOrigin.JAVA_SYNTHETIC_PROPERTY) return false
        val n = p.name.asString()
        val cap = n.replaceFirstChar { it.uppercaseChar() }
        val isName = n.startsWith("is") && n.length > 2 && !n[2].isLowerCase()
        return when (d.valueParameters.size) {
            0 -> d.name.asString() == if (isName) n else "get$cap"
            1 -> !p.isVal && d.name.asString() == "set" + if (isName) n.substring(2) else cap
            else -> false
        }
    }

    /** Whether [m]'s substitutor changes none of its types. */
    private fun unchanged(m: Candidate): Boolean {
        val s = m.symbol
        if (m.sub.substituteOrNull(s.returnType) != null) return false
        if (s.receiverType?.let(m.sub::substituteOrNull) != null) return false
        return s !is KaFunctionSymbol || s.valueParameters.none { m.sub.substituteOrNull(it.returnType) != null }
    }

    private fun fake(m: Candidate, origin: KaSymbolOrigin, intersected: List<KaCallableSymbol>, key: String): KaCallableSymbol =
        when (val s = m.symbol) {
            is KaNamedFunctionSymbol -> KcFakeFunctionSymbol(c, s, m.sub, origin, intersected, key)
            is KaKotlinPropertySymbol -> KcFakePropertySymbol(c, s, m.sub, origin, intersected, key)
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
                sameType(a.receiverType, x.sub, b.receiverType, sb) &&
                    a.valueParameters.indices.all { sameType(a.valueParameters[it].returnType, x.sub, b.valueParameters[it].returnType, sb) }
            }
            a is KaPropertySymbol && b is KaPropertySymbol ->
                a.name == b.name && (a.receiverParameter == null) == (b.receiverParameter == null) &&
                    sameType(a.receiverType, x.sub, b.receiverType, positional(y.sub, b.typeParameters, a.typeParameters))
            a is KaJavaFieldSymbol && b is KaJavaFieldSymbol -> a.name == b.name
            else -> false
        }
    }

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

/** A function a class inherits under a new signature: a substitution or intersection override. */
internal class KcFakeFunctionSymbol(
    override val owner: KcClassSymbol,
    override val original: KaNamedFunctionSymbol,
    private val sub: KaSubstitutor,
    override val origin: KaSymbolOrigin,
    override val intersected: List<KaCallableSymbol>,
    private val key: String,
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
    private val sub: KaSubstitutor,
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
