package io.github.barteo.kc.aa

import com.intellij.openapi.project.Project
import com.intellij.psi.PsiElement
import com.intellij.psi.search.GlobalSearchScope
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.projectStructure.KaModule
import org.jetbrains.kotlin.analysis.api.scopes.KaScope
import org.jetbrains.kotlin.analysis.api.session.KaSessionProvider
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaDeclarationContainerSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.psi.*
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
class KcAnalysis internal constructor(internal val round: KcRound, private val module: KaModule) {
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
    fun findClass(classId: ClassId): KaClassSymbol? = round.classes[classId] ?: round.libraryClass(classId)
    fun findClassLike(classId: ClassId): KaClassLikeSymbol? =
        round.classes[classId] ?: round.typeAliases[classId] ?: round.libraryClass(classId)
    /** A source type alias; library aliases aren't known yet. */
    fun findTypeAlias(classId: ClassId): KaTypeAliasSymbol? = round.typeAliases[classId]

    // KaScopeProvider
    fun getFileScope(file: KaFileSymbol): KaScope = KcScope((file as KcFileSymbol).declarations, round.token)
    fun getDeclaredMemberScope(s: KaDeclarationContainerSymbol): KaScope =
        KcScope(members(s).filterNot { isStatic(it) }, round.token)
    fun getStaticDeclaredMemberScope(s: KaDeclarationContainerSymbol): KaScope =
        KcScope(members(s).filter { isStatic(it) }, round.token)
    fun getCombinedDeclaredMemberScope(s: KaDeclarationContainerSymbol): KaScope = KcScope(members(s), round.token)
    // TODO(M4): inherited Java statics.
    fun getStaticMemberScope(s: KaDeclarationContainerSymbol): KaScope = getStaticDeclaredMemberScope(s)

    // TODO(M4): inherited members and fake overrides.
    fun getMemberScope(s: KaDeclarationContainerSymbol): KaScope = when (s) {
        is KcClassSymbol -> getDeclaredMemberScope(s)
        is KcLibraryClassSymbol -> KcScope(emptyList(), round.token)
        else -> unsupported("member scope of $s")
    }

    // KaEvaluator: only the expressions kc evaluated, annotation arguments
    // and annotation parameters' defaults.
    fun evaluateAsAnnotationValue(expr: KtExpression): KaAnnotationValue? = round.values[expr]

    // KaSymbolRelationProvider
    fun getContainingSymbol(s: KaSymbol): KaSymbol? = when (s) {
        is KcClassSymbol -> s.info.parent.takeUnless { it is KaFileSymbol }
        is KcFunctionSymbol -> s.info.parent.takeUnless { it is KaFileSymbol }
        is KcPropertySymbol -> s.info.parent.takeUnless { it is KaFileSymbol }
        is KcTypeAliasSymbol -> s.info.parent.takeUnless { it is KaFileSymbol }
        is KcConstructorSymbol -> s.info.parent
        is KcEnumEntrySymbol -> s.info.parent
        is KcValueParameterSymbol -> s.info.parent
        is KcTypeParameterSymbol -> s.info.parent
        is KcJavaFieldSymbol -> s.info.parent
        is KcGetterSymbol -> s.property
        is KcSetterSymbol -> s.property
        is KcLibraryClassSymbol -> null
        else -> unsupported("containingSymbol of $s")
    }
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
    /** kc expands type aliases in every type it reports. */
    fun getFullyExpandedType(type: KaType): KaType = type
    fun getExpandedSymbol(type: KaType): KaClassSymbol? = when (type) {
        is KaClassType -> type.symbol as? KaClassSymbol
        is KcFlexibleType -> getExpandedSymbol(type.lowerBound)
        else -> null
    }
    fun isMarkedNullable(type: KaType): Boolean = base(type).marked
    fun isNullable(type: KaType): Boolean = when (type) {
        is KcFlexibleType -> isNullable(type.upperBound)
        is KcDefinitelyNotNullType -> false
        is KcTypeParameterType -> type.marked || type.symbol.upperBounds.all(::isNullable)
        else -> base(type).marked
    }
    fun hasFlexibleNullability(type: KaType): Boolean =
        type is KcFlexibleType && isMarkedNullable(type.upperBound) && !isMarkedNullable(type.lowerBound)
    fun isAnyType(type: KaType): Boolean = type is KcClassType && !type.marked && type.classId == ANY

    // KaBuiltinTypesProvider
    fun getBuiltinTypes(): org.jetbrains.kotlin.analysis.api.components.KaBuiltinTypes = builtins
    private val builtins by lazy { KcBuiltinTypes(round) }

    // KaTypeCreator
    fun withNullability(type: KaType, isMarkedNullable: Boolean): KaType = base(type).withNullability(isMarkedNullable)
    fun lowerBoundIfFlexible(type: KaType): KaType = (type as? KcFlexibleType)?.lowerBound ?: type
    fun upperBoundIfFlexible(type: KaType): KaType = (type as? KcFlexibleType)?.upperBound ?: type
    fun buildClassType(classId: ClassId, init: Function1<Any?, Unit>): KaType =
        KcClassTypeBuilder(round.token).also { init(it) }.build(round, classId)
    fun buildClassType(symbol: KaClassLikeSymbol, init: Function1<Any?, Unit>): KaType =
        KcClassTypeBuilder(round.token).also { init(it) }.build(round, symbol.classId ?: unsupported("class type of local $symbol"))

    // KaTypeRelationChecker
    fun getDirectSupertypes(type: KaType): Sequence<KaType> = directSupertypes(type, true)
    fun directSupertypes(type: KaType, shouldApproximate: Boolean): Sequence<KaType> {
        // TODO(M4): substitute the class's type parameters with the type's arguments.
        val s = getExpandedSymbol(type) ?: return emptySequence()
        return s.superTypes.asSequence()
    }

    private fun base(type: KaType): KcTypeBase = type as? KcTypeBase ?: unsupported("type $type isn't kc's")

    private fun members(s: KaDeclarationContainerSymbol): List<KaDeclarationSymbol> =
        (s as? KcClassSymbol)?.members ?: unsupported("members of $s")

    /**
     * Whether a member belongs in the static scope: a nested class that
     * isn't inner, an enum entry, or a Java class's static member.
     */
    private fun isStatic(s: KaDeclarationSymbol) = when (s) {
        is KcClassSymbol -> !s.isInner
        is KcEnumEntrySymbol -> true
        is KcFunctionSymbol -> s.isStatic
        is KcPropertySymbol -> s.isStatic
        is KcJavaFieldSymbol -> s.isStatic
        else -> false
    }

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
