package io.github.barteo.kc.aa

import com.intellij.openapi.project.Project
import com.intellij.psi.PsiClass
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiMember
import com.intellij.psi.search.GlobalSearchScope
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.projectStructure.KaModule
import org.jetbrains.kotlin.analysis.api.scopes.KaScope
import org.jetbrains.kotlin.analysis.api.session.KaSessionProvider
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaDeclarationContainerSymbol
import org.jetbrains.kotlin.analysis.api.signatures.KaCallableSignature
import org.jetbrains.kotlin.analysis.api.signatures.KaFunctionSignature
import org.jetbrains.kotlin.analysis.api.signatures.KaVariableSignature
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.analysis.api.types.KaSubstitutor
import org.jetbrains.kotlin.analysis.api.components.KaSubtypingErrorTypePolicy
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.load.kotlin.TypeMappingMode
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.*
import org.jetbrains.org.objectweb.asm.Type
import java.lang.reflect.InvocationHandler
import java.lang.reflect.InvocationTargetException
import java.lang.reflect.Method
import java.lang.reflect.Proxy
import java.util.concurrent.ConcurrentHashMap

/**
 * The `KaSession` members kc implements. A [KaSession] is a proxy (see
 * [session]) that calls the method of this class with the interface
 * method's name and parameter types; extension members take their receiver
 * first, as they compile. Any other abstract member throws
 * `UnsupportedOperationException("kc: KaSession.name(...)")`, and default
 * members run their default body.
 */
@Suppress("unused") // called reflectively
class KcAnalysis internal constructor(
    internal val round: KcRound,
    private val module: KaModule,
    private val libraryModule: KaModule,
) {
    fun getToken(): KaLifetimeToken = round.token
    fun getUseSiteModule(): KaModule = module
    /** kc has no IntelliJ search scope: KSP's Java class lookups find nothing and fall back to [KcExtras]. */
    fun getAnalysisScope(): GlobalSearchScope = GlobalSearchScope.EMPTY_SCOPE

    // KaSymbolProvider
    fun getSymbol(file: KtFile): KaFileSymbol = round.byPsi[file] as? KaFileSymbol ?: noSymbol(file)
    fun getSymbol(declaration: KtDeclaration): KaDeclarationSymbol =
        round.byPsi[declaration] as? KaDeclarationSymbol ?: noSymbol(declaration)
    fun getSymbol(c: KtClassOrObject): KaClassSymbol? = round.byPsi[c] as? KaClassSymbol
    fun getNamedClassSymbol(c: KtClassOrObject): KaNamedClassSymbol? = round.byPsi[c] as? KaNamedClassSymbol
    // KaJavaInteroperabilityComponent: the Java sources' classes and members.
    fun getNamedClassSymbol(c: PsiClass): KaNamedClassSymbol? {
        synchronized(round) { round.byPsi[c] }?.let { return it as? KaNamedClassSymbol }
        round.libraryClass(round.javaClassId(c) ?: return null)
        return synchronized(round) { round.byPsi[c] } as? KaNamedClassSymbol
    }
    fun getCallableSymbol(m: PsiMember): KaCallableSymbol? {
        m.containingClass?.let(::getNamedClassSymbol)
        return synchronized(round) { round.byPsi[m] } as? KaCallableSymbol
    }

    fun findClass(classId: ClassId): KaClassSymbol? = round.classes[classId] ?: round.libraryClass(classId)
    fun findClassLike(classId: ClassId): KaClassLikeSymbol? =
        round.classes[classId] ?: round.typeAliases[classId] ?: round.libraryClass(classId) ?: round.typeAlias(classId)
    fun findTypeAlias(classId: ClassId): KaTypeAliasSymbol? = round.typeAlias(classId)
    fun findPackage(fqName: FqName): KaPackageSymbol? = if (round.packageExists(fqName)) round.packageSymbol(fqName) else null
    fun findTopLevelCallables(packageFqName: FqName, name: Name): Sequence<KaCallableSymbol> =
        round.topLevelCallables(packageFqName, name).asSequence()

    // KaScopeProvider
    fun getFileScope(file: KaFileSymbol): KaScope = KcScope((file as KcFileSymbol).declarations, round.token)
    /** The package's declarations; no subpackages (`getPackageSymbols` is empty). */
    fun getPackageScope(p: KaPackageSymbol): KaScope = KcScope(round.packageDeclarations(p.fqName), round.token)
    fun getDeclaredMemberScope(s: KaDeclarationContainerSymbol): KaScope =
        KcScope(members(s).filterNot { isStatic(it) }, round.token)
    fun getStaticDeclaredMemberScope(s: KaDeclarationContainerSymbol): KaScope =
        KcScope(members(s).filter { isStatic(it) }, round.token)
    fun getCombinedDeclaredMemberScope(s: KaDeclarationContainerSymbol): KaScope = KcScope(members(s), round.token)
    fun getStaticMemberScope(s: KaDeclarationContainerSymbol): KaScope = when (s) {
        is KcClassSymbol -> KcScope(round.members(s).statics, round.token)
        is KcLibraryClassSymbol -> KcScope(emptyList(), round.token)
        else -> unsupported("static member scope of $s")
    }

    /** Declared and inherited non-static callables, non-static nested classes and constructors. */
    fun getMemberScope(s: KaDeclarationContainerSymbol): KaScope = when (s) {
        is KcClassSymbol -> KcScope(
            round.members(s).callables + s.members.filter { it is KaConstructorSymbol || it is KaClassSymbol && !isStatic(it) },
            round.token,
        )
        is KcLibraryClassSymbol -> KcScope(emptyList(), round.token)
        else -> unsupported("member scope of $s")
    }

    // KaEvaluator: only the expressions kc evaluated, annotation arguments
    // and annotation parameters' defaults.
    fun evaluateAsAnnotationValue(expr: KtExpression): KaAnnotationValue? = round.values[expr]

    // KaSymbolRelationProvider
    fun getContainingSymbol(s: KaSymbol): KaSymbol? = when (s) {
        is KcFakeOverride -> s.owner
        is KcFakeGetterSymbol -> s.property
        is KcFakeSetterSymbol -> s.property
        is KcFakeValueParameterSymbol -> s.owner
        is KcFakeReceiverParameterSymbol -> s.owningCallableSymbol
        is KcClassSymbol -> s.info.parent.takeUnless { it is KaFileSymbol || it is KaPackageSymbol }
        is KcFunctionSymbol -> s.info.parent.takeUnless { it is KaFileSymbol || it is KaPackageSymbol }
        is KcPropertySymbol -> s.info.parent.takeUnless { it is KaFileSymbol || it is KaPackageSymbol }
        is KcTypeAliasSymbol -> s.info.parent.takeUnless { it is KaFileSymbol || it is KaPackageSymbol }
        is KcConstructorSymbol -> s.info.parent
        is KcEnumEntrySymbol -> s.info.parent
        is KcValueParameterSymbol -> s.info.parent
        is KcTypeParameterSymbol -> s.info.parent
        is KcJavaFieldSymbol -> s.info.parent
        is KcGetterSymbol -> s.property
        is KcSetterSymbol -> s.property
        is KcLibraryClassSymbol -> null
        is KcPackageSymbol -> null
        is KcReceiverParameterSymbol -> s.owningCallableSymbol
        else -> unsupported("containingSymbol of $s")
    }

    /** The declaration [s] is in: its containing symbol, unless that's a file or package. */
    fun getContainingDeclaration(s: KaSymbol): KaDeclarationSymbol? = getContainingSymbol(s) as? KaDeclarationSymbol

    /** The source module for source symbols, the library module for library ones. */
    fun getContainingModule(s: KaSymbol): KaModule = when (s.origin) {
        KaSymbolOrigin.LIBRARY, KaSymbolOrigin.JAVA_LIBRARY -> libraryModule
        KaSymbolOrigin.SUBSTITUTION_OVERRIDE, KaSymbolOrigin.INTERSECTION_OVERRIDE, KaSymbolOrigin.DELEGATED ->
            getContainingSymbol(s)?.let(::getContainingModule) ?: module
        else -> module
    }

    fun getDirectlyOverriddenSymbols(s: KaCallableSymbol): Sequence<KaCallableSymbol> = round.directlyOverridden(s).asSequence()
    fun getAllOverriddenSymbols(s: KaCallableSymbol): Sequence<KaCallableSymbol> = round.allOverridden(s).asSequence()
    fun getIntersectionOverriddenSymbols(s: KaCallableSymbol): List<KaCallableSymbol> =
        (s as? KcFakeOverride)?.intersected.orEmpty()
    fun getFakeOverrideOriginal(s: KaCallableSymbol): KaCallableSymbol = fakeOverrideOriginal(s)
    fun isSubClassOf(sub: KaClassSymbol, sup: KaClassSymbol): Boolean = round.relations.isSubClass(sub, sup)
    fun isDirectSubClassOf(sub: KaClassSymbol, sup: KaClassSymbol): Boolean = round.relations.isDirectSubClass(sub, sup)

    /** The source classes that directly extend sealed class [s], in source order. */
    fun getSealedClassInheritors(s: KaNamedClassSymbol): List<KaNamedClassSymbol> {
        if (s is KcClassSymbol && s.info.origin == KaSymbolOrigin.LIBRARY) {
            return s.info.decl.inheritors.mapNotNull { round.libraryClass(ClassId.fromString(it)) }
        }
        val out = ArrayList<KaNamedClassSymbol>()
        fun walk(ds: List<KaDeclarationSymbol>) {
            for (d in ds) if (d is KcClassSymbol) {
                if (d !== s && round.relations.isDirectSubClass(d, s)) out += d
                walk(d.members)
            }
        }
        for (f in round.fileSymbols) walk(f.declarations)
        return out
    }

    // KaVisibilityChecker
    /** Whether [s] is visible in [cls]: a private member only in its own class. */
    fun isVisibleInClass(s: KaCallableSymbol, cls: KaClassSymbol): Boolean =
        s.visibility != KaSymbolVisibility.PRIVATE || getContainingSymbol(fakeOverrideOriginal(s)) == cls

    // KaSubstitutorProvider and KaSignatureSubstitutor
    fun createSubstitutor(map: Map<KaTypeParameterSymbol, KaType>): KaSubstitutor = round.relations.substitutor(map)
    fun createInheritanceTypeSubstitutor(sub: KaClassSymbol, sup: KaClassSymbol): KaSubstitutor? =
        round.relations.inheritanceSubstitutor(sub, sup)
    fun substitute(s: KaCallableSymbol, sub: KaSubstitutor): KaCallableSignature<KaCallableSymbol> =
        round.relations.signature(s).substitute(sub)
    fun substitute(s: KaFunctionSymbol, sub: KaSubstitutor): KaFunctionSignature<KaFunctionSymbol> =
        round.relations.functionSignature(s).substitute(sub)
    fun substitute(s: KaVariableSymbol, sub: KaSubstitutor): KaVariableSignature<KaVariableSymbol> =
        round.relations.variableSignature(s).substitute(sub)
    fun asSignature(s: KaCallableSymbol): KaCallableSignature<KaCallableSymbol> = round.relations.signature(s)
    fun asSignature(s: KaFunctionSymbol): KaFunctionSignature<KaFunctionSymbol> = round.relations.functionSignature(s)
    fun asSignature(s: KaVariableSymbol): KaVariableSignature<KaVariableSymbol> = round.relations.variableSignature(s)
    fun getContainingFile(s: KaSymbol): KaFileSymbol? {
        var cur: KaSymbol? = s
        while (cur != null && cur !is KaFileSymbol) {
            cur = when (cur) {
                is KcClassSymbol -> cur.info.parent
                is KcFunctionSymbol -> cur.info.parent
                is KcPropertySymbol -> cur.info.parent
                is KcTypeAliasSymbol -> cur.info.parent
                else -> getContainingSymbol(cur)
            }
        }
        return cur as KaFileSymbol?
    }

    // KaTypeProvider
    fun getType(ref: KtTypeReference): KaType {
        val entry = (ref.parent as? KtConstructorCalleeExpression)?.parent as? KtAnnotationEntry
        if (entry != null) {
            val id = round.annotationClasses[entry] ?: return KcErrorType(round, ref.text)
            return KcClassType(round, id)
        }
        return round.typeOf(ref) ?: unsupported("type of ${ref.text}: not a declaration's type")
    }

    // KaTypeInformationProvider
    /**
     * kc expands type aliases in every type it reports, but the
     * abbreviations KSP takes of them (`abbreviationOrSelf`) are typealias
     * types.
     */
    fun getFullyExpandedType(type: KaType): KaType = round.expand(type)

    // KaJavaInteroperabilityComponent.
    private val jvmTypes by lazy { KcJvmTypes(round) }
    fun mapToJvmTypeDescriptor(type: KaType): String = jvmTypes.descriptor(type)
    fun mapToJvmType(type: KaType, mode: TypeMappingMode): Type = Type.getType(jvmTypes.descriptor(type))
    fun isPrimitiveBacked(type: KaType): Boolean = jvmTypes.descriptor(type).length == 1
    fun getExpandedSymbol(type: KaType): KaClassSymbol? = when (val type = round.expand(type)) {
        is KaClassType -> type.symbol as? KaClassSymbol
        is KcFlexibleType -> getExpandedSymbol(type.lowerBound)
        else -> null
    }
    fun isMarkedNullable(type: KaType): Boolean = base(type).marked
    fun isNullable(type: KaType): Boolean = round.relations.isNullable(type)
    fun hasFlexibleNullability(type: KaType): Boolean =
        type is KcFlexibleType && isMarkedNullable(type.upperBound) && !isMarkedNullable(type.lowerBound)
    fun isAnyType(type: KaType): Boolean = round.expand(type).let { it is KcClassType && !it.marked && it.classId == ANY }

    // KaBuiltinTypesProvider
    fun getBuiltinTypes(): org.jetbrains.kotlin.analysis.api.components.KaBuiltinTypes = builtins
    private val builtins by lazy { KcBuiltinTypes(round) }

    // KaTypeCreator
    fun withNullability(type: KaType, isMarkedNullable: Boolean): KaType = base(type).withNullability(isMarkedNullable)
    fun lowerBoundIfFlexible(type: KaType): KaType = (type as? KcFlexibleType)?.lowerBound ?: type
    fun upperBoundIfFlexible(type: KaType): KaType = (type as? KcFlexibleType)?.upperBound ?: type
    fun buildClassType(classId: ClassId, init: Function1<Any?, Unit>): KaType =
        KcClassTypeBuilder(round.token).also { init(it) }.build(round, classId)
    fun buildClassType(symbol: KaClassLikeSymbol, init: Function1<Any?, Unit>): KaType {
        val b = KcClassTypeBuilder(round.token).also { init(it) }
        return if (symbol is KaTypeAliasSymbol && symbol.classId != null) b.build(round, symbol)
        else b.build(round, symbol.classId ?: unsupported("class type of local $symbol"))
    }

    fun buildTypeParameterType(symbol: KaTypeParameterSymbol, init: Function1<Any?, Unit>): KaType {
        val b = KcTypeParameterTypeBuilder(round.token).also { init(it) }
        return KcTypeParameterType(round, symbol.name, symbol, b.isMarkedNullable)
    }

    // KaTypeRelationChecker
    fun getDirectSupertypes(type: KaType): Sequence<KaType> = directSupertypes(type, true)
    fun directSupertypes(type: KaType, shouldApproximate: Boolean): Sequence<KaType> =
        round.relations.directSupertypes(type).asSequence()
    fun semanticallyEquals(a: KaType, b: KaType, policy: KaSubtypingErrorTypePolicy): Boolean =
        round.relations.equal(a, b, policy == KaSubtypingErrorTypePolicy.LENIENT)
    fun isSubtypeOf(a: KaType, b: KaType, policy: KaSubtypingErrorTypePolicy): Boolean =
        round.relations.isSubtype(a, b, policy == KaSubtypingErrorTypePolicy.LENIENT)
    fun isSubtypeOf(a: KaType, id: ClassId, policy: KaSubtypingErrorTypePolicy): Boolean =
        round.relations.isSubtype(a, id, policy == KaSubtypingErrorTypePolicy.LENIENT)
    fun isSubtypeOf(a: KaType, s: KaClassLikeSymbol, policy: KaSubtypingErrorTypePolicy): Boolean =
        s.classId?.let { round.relations.isSubtype(a, it, policy == KaSubtypingErrorTypePolicy.LENIENT) } ?: false

    private fun base(type: KaType): KcTypeBase = type as? KcTypeBase ?: unsupported("type $type isn't kc's")

    private fun members(s: KaDeclarationContainerSymbol): List<KaDeclarationSymbol> =
        (s as? KcClassSymbol)?.members ?: unsupported("members of $s")

    /**
     * Whether a member belongs in the static scope: a nested class that
     * isn't inner, an enum entry, or a Java class's static member.
     */
    private fun isStatic(s: KaDeclarationSymbol) = isStaticMember(s)

    private fun noSymbol(e: PsiElement): Nothing = unsupported("no symbol for ${e.javaClass.simpleName} ${e.text.take(40)}")

    companion object {
        private val dispatch = ConcurrentHashMap<Method, Method?>()

        private val ANY = ClassId.fromString("kotlin/Any")

        internal fun unsupported(what: String): Nothing = throw UnsupportedOperationException("kc: $what")

        /** The implementation behind a session made by [session]. */
        internal fun of(session: KaSession): KcAnalysis =
            (Proxy.getInvocationHandler(session) as Handler).analysis

        internal fun session(analysis: KcAnalysis): KaSession =
            Proxy.newProxyInstance(KaSession::class.java.classLoader, arrayOf(KaSession::class.java), Handler(analysis)) as KaSession

        private class Handler(val analysis: KcAnalysis) : InvocationHandler {
            override fun invoke(proxy: Any, method: Method, args: Array<out Any?>?): Any? {
                val a = args ?: emptyArray()
                when (method.name) {
                    "equals" -> if (a.size == 1 && method.parameterTypes[0] == Any::class.java) return proxy === a[0]
                    "hashCode" -> if (a.isEmpty()) return System.identityHashCode(proxy)
                    "toString" -> if (a.isEmpty()) return "KcSession"
                }
                val impl = dispatch.computeIfAbsent(method) {
                    try {
                        KcAnalysis::class.java.getMethod(it.name, *it.parameterTypes)
                    } catch (_: NoSuchMethodException) {
                        null
                    }
                }
                try {
                    return when {
                        impl != null -> impl.invoke(analysis, *a)
                        method.isDefault -> InvocationHandler.invokeDefault(proxy, method, *a)
                        else -> unsupported(
                            "KaSession.${method.name}(${method.parameterTypes.joinToString { it.simpleName }})",
                        )
                    }
                } catch (e: InvocationTargetException) {
                    throw e.targetException
                }
            }
        }
    }
}

/** Hands out the current round's session for every `analyze` call. */
internal class KcSessionProvider(project: Project) : KaSessionProvider(project) {
    @Volatile
    lateinit var session: KaSession

    override fun getAnalysisSession(useSiteElement: PsiElement): KaSession = session
    override fun getAnalysisSession(useSiteModule: KaModule): KaSession = session
    override fun beforeEnteringAnalysis(session: KaSession, useSiteElement: PsiElement) {}
    override fun beforeEnteringAnalysis(session: KaSession, useSiteModule: KaModule) {}
    override fun afterLeavingAnalysis(session: KaSession, useSiteElement: PsiElement) {}
    override fun afterLeavingAnalysis(session: KaSession, useSiteModule: KaModule) {}
    override fun handleAnalysisException(throwable: Throwable, session: KaSession, useSiteElement: PsiElement): Nothing =
        throw throwable
    override fun handleAnalysisException(throwable: Throwable, session: KaSession, useSiteModule: KaModule): Nothing =
        throw throwable
    override fun clearCaches() {}
}
