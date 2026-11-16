@file:Suppress("DEPRECATION", "DEPRECATION_ERROR", "unused")

package io.github.barteo.kc.detekt

import com.intellij.psi.PsiElement
import com.intellij.psi.search.GlobalSearchScope
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.components.KaDiagnosticCheckerFilter
import org.jetbrains.kotlin.analysis.api.components.KaScopeContext
import org.jetbrains.kotlin.analysis.api.components.KaScopeImplicitValue
import org.jetbrains.kotlin.analysis.api.components.KaScopeKind
import org.jetbrains.kotlin.analysis.api.components.KaScopeWithKind
import org.jetbrains.kotlin.analysis.api.components.KaSubtypingErrorTypePolicy
import org.jetbrains.kotlin.analysis.api.diagnostics.KaDiagnosticWithPsi
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.projectStructure.KaModule
import org.jetbrains.kotlin.analysis.api.renderer.declarations.KaDeclarationRenderer
import org.jetbrains.kotlin.analysis.api.resolution.*
import org.jetbrains.kotlin.analysis.api.scopes.KaScope
import org.jetbrains.kotlin.analysis.api.signatures.KaFunctionSignature
import org.jetbrains.kotlin.analysis.api.signatures.KaVariableSignature
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaDeclarationContainerSymbol
import org.jetbrains.kotlin.analysis.api.symbols.pointers.KaSymbolPointer
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.idea.references.KtReference
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.*
import org.jetbrains.kotlin.resolution.KtResolvable
import org.jetbrains.kotlin.resolution.KtResolvableCall
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaNamedSymbol
import org.jetbrains.kotlin.psi.psiUtil.getParentOfType
import java.lang.reflect.InvocationHandler
import java.lang.reflect.InvocationTargetException
import java.lang.reflect.Method
import java.lang.reflect.Proxy
import java.util.concurrent.ConcurrentHashMap

/**
 * The `KaSession` members kc answers. A [KaSession] is a proxy (see
 * [session]) that calls the method of this class with the interface
 * method's name and parameter types; extension members take their receiver
 * first, as they compile. A default member runs its default body, and any
 * other member is reported to [KcMissing].
 */
class KcAnalysis internal constructor(private val s: KcSession) {
    fun getToken(): KaLifetimeToken = KcToken
    fun getAnalysisScope(): GlobalSearchScope = GlobalSearchScope.allScope(s.project)

    // KaExpressionTypeProvider

    fun getExpressionType(e: KtExpression): KaType? {
        val target = when (e) {
            is KtParenthesizedExpression -> return e.expression?.let(::getExpressionType)
            is KtLabeledExpression -> return e.baseExpression?.let(::getExpressionType)
            is KtAnnotatedExpression -> return e.baseExpression?.let(::getExpressionType)
            // A call's callee has the call's type.
            is KtNameReferenceExpression -> (e.parent as? KtCallExpression)?.takeIf { it.calleeExpression == e } ?: e
            else -> e
        }
        val f = s.fact(target) ?: return null
        val t = f.type ?: return null
        return s.type(t, nonNull = f.flowNonNull && !t.parameter)
    }

    fun getType(ref: KtTypeReference): KaType = typeOfRef(s, ref) ?: KcErrorType(s, ref.text)

    fun getReturnType(d: KtDeclaration): KaType = when (val sym = getSymbol(d)) {
        is KaCallableSymbol -> sym.returnType
        else -> KcErrorType(s, "return type of ${d.name}")
    }

    fun getExpectedType(e: PsiElement): KaType? = KcMissing.miss("KaSession.expectedType", null)

    fun isUsedAsExpression(e: KtExpression): Boolean = s.fact(e)?.usedAsExpression ?: KcMissing.miss("KaSession.isUsedAsExpression without facts", true)

    // KaSymbolProvider

    fun getSymbol(d: KtDeclaration): KaDeclarationSymbol =
        s.declarationSymbol(d) ?: KcMissing.fail("no symbol for ${d.javaClass.simpleName} ${d.name}")
    fun getSymbol(d: KtNamedFunction): KaFunctionSymbol = getSymbol(d as KtDeclaration) as KaFunctionSymbol
    fun getSymbol(d: KtProperty): KaVariableSymbol = getSymbol(d as KtDeclaration) as KaVariableSymbol
    fun getSymbol(d: KtParameter): KaVariableSymbol = getSymbol(d as KtDeclaration) as KaVariableSymbol
    fun getSymbol(d: KtFunctionLiteral): KaAnonymousFunctionSymbol = getSymbol(d as KtDeclaration) as KaAnonymousFunctionSymbol
    fun getSymbol(d: KtObjectLiteralExpression): KaAnonymousObjectSymbol = s.declarationSymbol(d) as KaAnonymousObjectSymbol
    fun getSymbol(d: KtObjectDeclaration): KaClassSymbol = getSymbol(d as KtDeclaration) as KaClassSymbol
    fun getSymbol(d: KtClassOrObject): KaClassSymbol? = s.declarationSymbol(d) as? KaClassSymbol
    fun getSymbol(d: KtDestructuringDeclaration): KaDestructuringDeclarationSymbol = getSymbol(d as KtDeclaration) as KaDestructuringDeclarationSymbol
    fun getSymbol(d: KtTypeParameter): KaTypeParameterSymbol = getSymbol(d as KtDeclaration) as KaTypeParameterSymbol
    fun getSymbol(d: KtConstructor<*>): KaConstructorSymbol = getSymbol(d as KtDeclaration) as KaConstructorSymbol
    fun getSymbol(d: KtTypeAlias): KaTypeAliasSymbol = getSymbol(d as KtDeclaration) as KaTypeAliasSymbol
    fun getSymbol(d: KtPropertyAccessor): KaPropertyAccessorSymbol = getSymbol(d as KtDeclaration) as KaPropertyAccessorSymbol
    fun getNamedClassSymbol(d: KtClassOrObject): KaNamedClassSymbol? = s.declarationSymbol(d) as? KaNamedClassSymbol

    fun findClass(id: ClassId): KaClassSymbol? = s.sourceClass(id)?.let { s.declarationSymbol(it) as? KaClassSymbol }
        ?: KcMissing.miss("KaSession.findClass of a library class", s.classSymbol(id) as? KaClassSymbol)
    fun findTypeAlias(id: ClassId): KaTypeAliasSymbol? = KcMissing.miss("KaSession.findTypeAlias", null)
    fun findTopLevelCallables(pkg: FqName, name: Name): Sequence<KaCallableSymbol> = KcMissing.miss("KaSession.findTopLevelCallables", emptySequence())
    fun restoreSymbol(p: KaSymbolPointer<*>): KaSymbol? = (p as? KcPointer<*>)?.restoreSymbol(KcAnalysis.session(this))

    // KaResolver

    fun resolveToCall(e: KtElement): KaCallInfo? = call(e)?.let(::KcSuccessCallInfo)
    fun resolveCall(e: KtResolvableCall): KaSingleOrMultiCall? = (e as? KtElement)?.let(::call) as? KaSingleOrMultiCall
    fun resolveCall(e: KtPropertyDelegate): KaDelegatedPropertyCall? = KcMissing.miss("KaSession.resolveCall(KtPropertyDelegate)", null)

    fun resolveSymbol(e: KtNameReferenceExpression): KaDeclarationSymbol? = referenced(e) as? KaDeclarationSymbol
    fun resolveSymbol(e: KtResolvable): KaSymbol? = when (e) {
        is KtSimpleNameExpression -> referenced(e)
        is KtQualifiedExpression -> e.selectorExpression?.let { resolveSymbol(it as KtResolvable) }
        is KtCallElement -> (call(e as KtElement) as? KaCallableMemberCall<*, *>)?.partiallyAppliedSymbol?.signature?.symbol
        is KtElement -> (call(e) as? KaCallableMemberCall<*, *>)?.partiallyAppliedSymbol?.signature?.symbol
        else -> null
    }
    fun resolveSymbols(e: KtResolvable): Collection<KaSymbol> = listOfNotNull(resolveSymbol(e))
    fun resolveToSymbol(r: KtReference): KaSymbol? = (r.element as? KtSimpleNameExpression)?.let(::referenced)
    fun resolveToSymbols(r: KtReference): Collection<KaSymbol> = listOfNotNull(resolveToSymbol(r))

    // KaTypeInformationProvider and KaTypeProvider

    private fun classFq(t: KaType): String? = when (t) {
        is KcClassType -> t.classId.asFqNameString()
        is KcFunctionType -> t.classId.asFqNameString()
        is KcFlexibleType -> classFq(t.lowerBound)
        else -> null
    }
    private fun isClass(t: KaType, fq: String) = classFq(t) == fq
    fun isUnitType(t: KaType) = isClass(t, "kotlin.Unit") && !isMarkedNullable(t)
    fun isIntType(t: KaType) = isClass(t, "kotlin.Int") && !isMarkedNullable(t)
    fun isLongType(t: KaType) = isClass(t, "kotlin.Long") && !isMarkedNullable(t)
    fun isCharType(t: KaType) = isClass(t, "kotlin.Char") && !isMarkedNullable(t)
    fun isBooleanType(t: KaType) = isClass(t, "kotlin.Boolean") && !isMarkedNullable(t)
    fun isStringType(t: KaType) = isClass(t, "kotlin.String") && !isMarkedNullable(t)
    fun isAnyType(t: KaType) = isClass(t, "kotlin.Any") && !isMarkedNullable(t)
    fun isNothingType(t: KaType) = isClass(t, "kotlin.Nothing") && !isMarkedNullable(t)
    fun isClassType(t: KaType, id: ClassId) = classFq(t) == id.asFqNameString()
    fun isPrimitive(t: KaType) = classFq(t) in primitives && !isMarkedNullable(t)
    fun isMarkedNullable(t: KaType) = (t as? KcTypeBase)?.marked ?: false
    fun isNullable(t: KaType) = (t as? KcTypeBase)?.holdsNull ?: false
    fun getHasFlexibleNullability(t: KaType) = t is KcFlexibleType
    fun isSuspendFunctionType(t: KaType) = classFq(t)?.startsWith("kotlin.coroutines.SuspendFunction") == true
    fun isFunctionType(t: KaType) = classFq(t)?.matches(Regex("""kotlin\.Function\d+""")) == true
    fun isFunctionalInterface(t: KaType): Boolean = (t as? KcClassType)?.kc?.sam ?: false
    fun isArrayOrPrimitiveArray(t: KaType) = classFq(t)?.let { it == "kotlin.Array" || it in primitiveArrays } == true
    fun getArrayElementType(t: KaType): KaType? = when (val fq = classFq(t)) {
        "kotlin.Array" -> (t as? KcClassType)?.typeArguments?.firstOrNull()?.type
        in primitiveArrays -> s.classType("kotlin/" + fq!!.removePrefix("kotlin.").removeSuffix("Array"), false)
        else -> null
    }
    fun upperBoundIfFlexible(t: KaType): KaType = (t as? KcFlexibleType)?.upperBound ?: t
    fun lowerBoundIfFlexible(t: KaType): KaType = (t as? KcFlexibleType)?.lowerBound ?: t
    fun getAllSupertypes(t: KaType): Sequence<KaType> = getAllSupertypes(t, false)
    fun getAllSupertypes(t: KaType, shouldApproximate: Boolean): Sequence<KaType> {
        val names = (t as? KcTypeBase)?.classNames ?: return emptySequence()
        val own = classFq(t)
        return names.asSequence().filter { it != own && !it.startsWith("java.") || it == "java.io.Serializable" }
            .distinct().map { s.classType(KcSession.fqClassId(it).asString(), false) }
    }
    fun getDirectSupertypes(t: KaType): Sequence<KaType> = getDirectSupertypes(t, false)
    fun getDirectSupertypes(t: KaType, shouldApproximate: Boolean): Sequence<KaType> =
        ((t as? KcClassType)?.symbol as? KaClassSymbol)?.superTypes?.asSequence() ?: emptySequence()
    fun getDefaultType(c: KaClassifierSymbol): KaType = when (c) {
        is KaClassSymbol -> c.classId?.let { id ->
            s.classType(id.asString(), false, c.typeParameters.map { KcTypeArgument(s, KcErrorType(s, it.name.asString()), org.jetbrains.kotlin.types.Variance.INVARIANT) })
        } ?: KcErrorType(s, "anonymous")
        else -> KcMissing.miss("KaSession.defaultType of ${c.javaClass.simpleName}", KcErrorType(s, c.toString()))
    }
    fun getExpandedSymbol(t: KaType): KaClassSymbol? = (t as? KcClassType)?.symbol as? KaClassSymbol
    fun getSymbol(t: KaType): KaClassLikeSymbol? = (t as? KcClassType)?.symbol
    fun withNullability(t: KaType, nullable: Boolean): KaType = (t as? KcTypeBase)?.withNullability(nullable) ?: t

    // KaTypeRelationChecker

    fun isSubtypeOf(a: KaType, b: KaType, policy: KaSubtypingErrorTypePolicy): Boolean = KcTypeRelations.isSubtype(a, b)
    fun isSubtypeOf(a: KaType, id: ClassId, policy: KaSubtypingErrorTypePolicy): Boolean = KcTypeRelations.isSubtype(a, id)
    fun isSubtypeOf(a: KaType, c: KaClassLikeSymbol, policy: KaSubtypingErrorTypePolicy): Boolean =
        c.classId?.let { KcTypeRelations.isSubtype(a, it) } ?: false
    fun isClassSubtypeOf(a: KaType, id: ClassId, policy: KaSubtypingErrorTypePolicy): Boolean = KcTypeRelations.isSubtype(a, id)
    fun semanticallyEquals(a: KaType, b: KaType, policy: KaSubtypingErrorTypePolicy): Boolean = a == b

    // KaDiagnosticProvider: kc reports no diagnostics for sources it compiles.

    fun directDiagnostics(e: KtElement, filter: KaDiagnosticCheckerFilter): Collection<KaDiagnosticWithPsi<*>> = emptyList()
    fun collectDiagnostics(f: KtFile, filter: KaDiagnosticCheckerFilter): Collection<KaDiagnosticWithPsi<*>> = emptyList()

    // KaSymbolRelationProvider and KaSymbolInformationProvider

    fun getContainingSymbol(sym: KaSymbol): KaSymbol? = getContainingDeclaration(sym)
    fun getContainingDeclaration(sym: KaSymbol): KaDeclarationSymbol? {
        if (sym is KcReceiverParameterSymbol) return sym.owningCallableSymbol
        val psi = sym.psi ?: return null
        var p = psi.parent
        while (p != null && p !is KtFile) {
            if (p is KtDeclaration && p !is KtDestructuringDeclaration || p is KtFunctionLiteral) {
                (if (p is KtFunctionLiteral) s.declarationSymbol(p) else s.declarationSymbol(p))?.let { return it }
            }
            p = p.parent
        }
        return null
    }
    fun getAllOverriddenSymbols(sym: KaCallableSymbol): Sequence<KaCallableSymbol> = KcMissing.miss("KaSession.allOverriddenSymbols", emptySequence())
    fun getDirectlyOverriddenSymbols(sym: KaCallableSymbol): Sequence<KaCallableSymbol> = KcMissing.miss("KaSession.directlyOverriddenSymbols", emptySequence())
    fun getFakeOverrideOriginal(sym: KaCallableSymbol): KaCallableSymbol = sym
    fun isPublicApi(sym: KaDeclarationSymbol): Boolean {
        var cur: KaDeclarationSymbol? = sym
        while (cur != null) {
            val v = when (cur) {
                is KaClassLikeSymbol -> (cur as? KaNamedClassSymbol)?.visibility ?: KaSymbolVisibility.LOCAL
                is KaCallableSymbol -> (cur as? KaDeclarationSymbol)?.let { visibility(it) } ?: KaSymbolVisibility.PUBLIC
                else -> KaSymbolVisibility.PUBLIC
            }
            // @PublishedApi internal declarations are public API, as inline functions' bodies see them.
            val published = v == KaSymbolVisibility.INTERNAL &&
                (cur.psi as? KtAnnotated)?.annotationEntries?.any { it.shortName?.asString() == "PublishedApi" } == true
            if (v != KaSymbolVisibility.PUBLIC && v != KaSymbolVisibility.PROTECTED && !published) return false
            cur = getContainingDeclaration(cur)
        }
        return true
    }
    private fun visibility(d: KaDeclarationSymbol): KaSymbolVisibility = when (d) {
        is KaNamedFunctionSymbol -> d.visibility
        is KaPropertySymbol -> d.visibility
        is KaConstructorSymbol -> d.visibility
        is KaLocalVariableSymbol, is KaValueParameterSymbol, is KaAnonymousFunctionSymbol -> KaSymbolVisibility.LOCAL
        else -> KaSymbolVisibility.PUBLIC
    }
    fun getAnnotationApplicableTargets(c: KaClassSymbol): Set<org.jetbrains.kotlin.descriptors.annotations.KotlinTarget>? =
        KcMissing.miss("KaSession.annotationApplicableTargets", null)

    // KaScopeProvider: a source class's own declarations.

    fun getDeclaredMemberScope(c: KaDeclarationContainerSymbol): KaScope = KcScope(members(c))
    fun getMemberScope(c: KaDeclarationContainerSymbol): KaScope {
        if (c !is KcSourceClassSymbol) KcMissing.miss("KaSession.memberScope of a library class", Unit)
        return KcScope(members(c))
    }
    private fun members(c: KaDeclarationContainerSymbol): List<KaDeclarationSymbol> {
        val cls = (c as? KaSymbol)?.psi as? KtClassOrObject ?: return emptyList()
        val out = ArrayList<KaDeclarationSymbol>()
        cls.primaryConstructor?.let { pc ->
            s.declarationSymbol(pc)?.let(out::add)
        }
        for (d in cls.declarations) {
            if (d is KtClassInitializer) continue
            s.declarationSymbol(d)?.let(out::add)
        }
        return out
    }

    // Scope contexts, from PSI: the declarations visible at a position,
    // innermost first, then the file's explicit imports and its package's
    // top-level declarations. kc lists no default imports.

    fun scopeContext(file: KtFile, position: KtElement): KaScopeContext {
        val scopes = ArrayList<KaScopeWithKind>()
        var child: PsiElement = position
        var p: PsiElement? = position.parent
        while (p != null && p !is KtFile) {
            val decls = ArrayList<KaDeclarationSymbol>()
            var kind: (Int) -> KaScopeKind = ::KcLocalScopeKind
            when (p) {
                is KtBlockExpression -> for (st in p.statements) {
                    if (st == child || st.textOffset >= child.textOffset) break
                    when (st) {
                        is KtDestructuringDeclaration -> st.entries.forEach { e -> s.declarationSymbol(e)?.let(decls::add) }
                        is KtNamedDeclaration -> s.declarationSymbol(st)?.let(decls::add)
                    }
                }
                is KtFunctionLiteral -> p.valueParameters.forEach { v -> s.declarationSymbol(v)?.let(decls::add) }
                is KtDeclarationWithBody -> {
                    p.valueParameters.forEach { v -> s.declarationSymbol(v)?.let(decls::add) }
                    (p as? KtTypeParameterListOwner)?.typeParameters?.forEach { t -> s.declarationSymbol(t)?.let(decls::add) }
                }
                is KtForExpression -> if (child == p.body) p.loopParameter?.let { v -> s.declarationSymbol(v)?.let(decls::add) }
                is KtCatchClause -> if (child == p.catchBody) p.catchParameter?.let { v -> s.declarationSymbol(v)?.let(decls::add) }
                is KtClassOrObject -> {
                    (s.declarationSymbol(p) as? KaDeclarationContainerSymbol)?.let { decls += members(it) }
                    p.typeParameters.forEach { t -> s.declarationSymbol(t)?.let(decls::add) }
                    kind = ::KcTypeScopeKind
                }
            }
            if (decls.isNotEmpty()) scopes += KcScopeWithKind(KcScope(decls), kind(scopes.size))
            child = p
            p = p.parent
        }
        scopes += importingScopes(file, scopes.size)
        return KcScopeContext(scopes)
    }

    fun asCompositeScope(scopes: List<KaScope>): KaScope = KcScope(scopes.flatMap { it.declarations.toList() })

    fun getImportingScopeContext(file: KtFile): KaScopeContext = KcScopeContext(importingScopes(file, 0))

    private fun importingScopes(file: KtFile, start: Int): List<KaScopeWithKind> {
        val imported = ArrayList<KaDeclarationSymbol>()
        for (d in file.importDirectives) {
            if (d.isAllUnder) continue
            val ref = when (val e = d.importedReference) {
                is KtQualifiedExpression -> e.selectorExpression
                else -> e
            } as? KtSimpleNameExpression
            val sym = ref?.let(::referenced) as? KaDeclarationSymbol
                ?: d.importedFqName?.let { fq -> s.sourceClass(ClassId.topLevel(fq))?.let { s.declarationSymbol(it) } }
            sym?.let(imported::add)
        }
        val pkg = s.packageDeclarations(file.packageFqName).mapNotNull { s.declarationSymbol(it) }
        return listOf(
            KcScopeWithKind(KcScope(imported), KcExplicitSimpleImportingScopeKind(start)),
            KcScopeWithKind(KcScope(pkg), KcPackageMemberScopeKind(start + 1)),
        )
    }

    // KaRenderer: kc renders a declaration as its PSI's text up to its body.

    fun render(sym: KaDeclarationSymbol, renderer: KaDeclarationRenderer): String {
        val psi = sym.psi as? KtDeclaration ?: return sym.toString()
        val body = (psi as? KtDeclarationWithBody)?.bodyExpression ?: (psi as? KtClassOrObject)?.body
        val text = if (body == null) psi.text else psi.text.substring(0, body.startOffsetInParent)
        return text.trim()
    }

    // Calls.

    /** The call [e] makes, as the Analysis API's resolveToCall finds it. */
    internal fun call(e: KtElement): KaCall? {
        when (e) {
            is KtQualifiedExpression -> return e.selectorExpression?.let(::call)
            is KtParenthesizedExpression -> return null
            is KtCallExpression -> return functionCall(e, s.fact(e)?.call ?: s.fact(e.calleeExpression)?.call)
            is KtSimpleNameExpression -> {
                val parent = e.parent
                if (parent is KtCallExpression && parent.calleeExpression == e) return call(parent)
                if (parent is KtBinaryExpression && parent.operationReference == e) return call(parent)
                if (parent is KtUnaryExpression && parent.operationReference == e) return call(parent)
                val f = s.fact(e) ?: return null
                if (f.classifier) return null
                return variableAccess(e, f) ?: f.call?.let { functionCall(e, it) }
            }
            is KtBinaryExpression -> {
                if (e.operationToken == KtTokens.EQ) {
                    val left = e.left?.let { if (it is KtQualifiedExpression) it.selectorExpression else it } ?: return null
                    return call(left)
                }
                return s.fact(e)?.call?.let { functionCall(e, it) }
            }
            is KtUnaryExpression, is KtArrayAccessExpression -> return s.fact(e)?.call?.let { functionCall(e, it) }
            else -> return s.fact(e)?.call?.let { functionCall(e, it) }
        }
    }

    /** The receiver [e]'s call passes explicitly, if it has one. */
    private fun explicitReceiver(e: KtElement): Pair<KtExpression, Boolean>? {
        val q = e.parent as? KtQualifiedExpression
        if (q != null && q.selectorExpression == e) return q.receiverExpression to (q is KtSafeQualifiedExpression)
        return when (e) {
            is KtBinaryExpression -> {
                val inOp = e.operationToken == KtTokens.IN_KEYWORD || e.operationToken == KtTokens.NOT_IN
                (if (inOp) e.right else e.left)?.let { it to false }
            }
            is KtUnaryExpression -> e.baseExpression?.let { it to false }
            is KtArrayAccessExpression -> e.arrayExpression?.let { it to false }
            else -> null
        }
    }

    private fun receiverValue(e: KtElement, implicit: KcElem?, passed: KcType?): KaReceiverValue? {
        explicitReceiver(e)?.let { (expr, safe) ->
            val f = s.fact(expr)
            val declared = f?.type ?: passed
            val explicit = KcExplicitReceiver(expr, safe, s.type(declared))
            return if (f?.flowNonNull == true) KcSmartCastReceiver(explicit, s.type(declared, nonNull = true)) else explicit
        }
        val psi = s.psi(implicit) ?: return null
        val owner: KaSymbol = when (psi) {
            is KtCallableDeclaration -> (s.declarationSymbol(psi) as? KaCallableSymbol)?.receiverParameter ?: s.declarationSymbol(psi)
            else -> s.declarationSymbol(psi)
        } ?: return null
        val type = passed?.let { s.type(it) } ?: (owner as? KaClassSymbol)?.let(::getDefaultType) ?: return null
        return KcImplicitReceiver(owner, type)
    }

    private fun functionCall(e: KtElement, c: KcCall?): KaCall? {
        if (c == null) return null
        val sym = s.callableSymbol(c) as? KaFunctionSymbol ?: return null
        val receiver = receiverValue(e, c.implicitReceiver, c.receiver)
        val params = sym.valueParameters
        val paramSigs = params.mapIndexed { i, p ->
            KcVariableSignature<KaValueParameterSymbol>(p, p.name, c.params.getOrNull(i)?.let { if (it.vararg) varargElement(s, it.type) else s.type(it.type) } ?: p.returnType)
        }
        val sig = KcFunctionSignature<KaFunctionSymbol>(sym, s.type(c.returnType ?: c.declaredReturnType), c.declaredReceiver?.let { s.type(it) }, paramSigs)
        val pas = if (c.extension) KcPartiallyApplied<KaFunctionSymbol, KaFunctionSignature<KaFunctionSymbol>>(sig, null, receiver)
        else KcPartiallyApplied(sig, receiver, null)
        val mapping = LinkedHashMap<KtExpression, KaVariableSignature<KaValueParameterSymbol>>()
        arguments(e).forEachIndexed { i, arg ->
            val p = c.argumentMapping.getOrNull(i) ?: return@forEachIndexed
            if (arg != null && p >= 0 && p < paramSigs.size) mapping[arg] = paramSigs[p]
        }
        return KcSimpleFunctionCall(pas, mapping, c.valueInvocation)
    }

    /** The arguments of [e]'s call, in kc's order: the value arguments, then a trailing lambda. */
    private fun arguments(e: KtElement): List<KtExpression?> = when (e) {
        is KtCallElement -> e.valueArguments.map { it.getArgumentExpression() }
        is KtSimpleNameExpression -> (e.parent as? KtCallExpression)?.let(::arguments) ?: emptyList()
        is KtBinaryExpression -> {
            val inOp = e.operationToken == KtTokens.IN_KEYWORD || e.operationToken == KtTokens.NOT_IN
            listOf(if (inOp) e.left else e.right)
        }
        is KtArrayAccessExpression -> e.indexExpressions
        else -> emptyList()
    }

    private fun variableAccess(e: KtSimpleNameExpression, f: KcFact): KaCall? {
        val sym = s.sym(f.symbol)
        val getter = f.accessors.firstOrNull()
        val vs: KaVariableSymbol = when {
            sym == null && getter == null -> return null
            sym != null && (sym.kind == KcSym.LOCAL || sym.kind == KcSym.PARAMETER) -> s.symbol(sym) as? KaVariableSymbol
            sym != null && sym.kind == KcSym.PROPERTY || getter != null -> s.propertySymbol(sym?.takeIf { it.kind == KcSym.PROPERTY }, getter) as? KaVariableSymbol
            else -> null
        } ?: return null
        val type = f.type?.let { s.type(it) } ?: vs.returnType
        val sig = KcVariableSignature<KaVariableSymbol>(vs, vs.name, type)
        val receiver = receiverValue(e, f.implicitReceiver, getter?.receiver)
        val extension = getter?.extension == true
        val pas = KcPartiallyApplied<KaVariableSymbol, KaVariableSignature<KaVariableSymbol>>(sig, if (extension) null else receiver, if (extension) receiver else null)
        return KcVariableAccessCall(pas, accessKind(e))
    }

    private fun accessKind(e: KtSimpleNameExpression): KaVariableAccessCall.Kind {
        var x: KtExpression = e
        (x.parent as? KtQualifiedExpression)?.takeIf { it.selectorExpression == x }?.let { x = it }
        val b = x.parent as? KtBinaryExpression
        return if (b != null && b.operationToken == KtTokens.EQ && b.left == x) KcWrite(b.right) else KcRead
    }

    /** The declaration a name refers to. */
    private fun referenced(e: KtSimpleNameExpression): KaSymbol? {
        val f = s.fact(e) ?: return null
        f.call?.let { c ->
            // A constructor call's callee names the constructor; a call's, the function.
            if (!c.valueInvocation) return s.callableSymbol(c, s.sym(f.symbol))
        }
        val sym = s.sym(f.symbol)
        if (sym == null) {
            f.accessors.firstOrNull()?.let { return s.propertySymbol(null, it) }
            return null
        }
        if (sym.kind == KcSym.PROPERTY) return s.propertySymbol(sym, f.accessors.firstOrNull())
        return s.symbol(sym)
    }

    companion object {
        private val primitives = setOf("kotlin.Int", "kotlin.Long", "kotlin.Short", "kotlin.Byte", "kotlin.Char", "kotlin.Boolean", "kotlin.Float", "kotlin.Double")
        private val primitiveArrays = primitives.map { it + "Array" }.toSet()

        private val dispatch = ConcurrentHashMap<Method, Method?>()

        internal fun session(analysis: KcAnalysis): KaSession =
            Proxy.newProxyInstance(KaSession::class.java.classLoader, arrayOf(KaSession::class.java), Handler(analysis)) as KaSession

        /** The method of this class [m] dispatches to: the same name and parameter types, or else parameter types [m]'s are assignable to. */
        private fun implementation(m: Method): Method? {
            try {
                return KcAnalysis::class.java.getMethod(m.name, *m.parameterTypes)
            } catch (_: NoSuchMethodException) {
            }
            return KcAnalysis::class.java.methods.filter { c ->
                c.name == m.name && c.parameterCount == m.parameterCount &&
                    c.parameterTypes.indices.all { i -> c.parameterTypes[i].isAssignableFrom(m.parameterTypes[i]) } &&
                    m.returnType.isAssignableFrom(c.returnType)
            }.minByOrNull { c -> c.parameterTypes.sumOf { p -> generateSequence<Class<*>>(p) { it.superclass }.count() } }
        }

        private class Handler(val analysis: KcAnalysis) : InvocationHandler {
            override fun invoke(proxy: Any, method: Method, args: Array<out Any?>?): Any? {
                val a = args ?: emptyArray()
                when (method.name) {
                    "equals" -> if (a.size == 1 && method.parameterTypes[0] == Any::class.java) return proxy === a[0]
                    "hashCode" -> if (a.isEmpty()) return System.identityHashCode(proxy)
                    "toString" -> if (a.isEmpty()) return "KcSession"
                }
                val impl = dispatch.computeIfAbsent(method) { implementation(it) }
                try {
                    return when {
                        impl != null -> impl.invoke(analysis, *a)
                        method.isDefault -> InvocationHandler.invokeDefault(proxy, method, *a)
                        else -> missing(method)
                    }
                } catch (e: InvocationTargetException) {
                    throw e.targetException
                }
            }

            private fun missing(m: Method): Any? {
                val what = "KaSession.${m.name}(${m.parameterTypes.joinToString { it.simpleName }})"
                val r = m.returnType
                val default: Any? = when {
                    r == java.lang.Boolean.TYPE -> false
                    r == Integer.TYPE -> 0
                    r == java.lang.Void.TYPE -> null
                    r.isPrimitive -> KcMissing.fail(what)
                    List::class.java.isAssignableFrom(r) || r == Collection::class.java -> emptyList<Any>()
                    r == Set::class.java -> emptySet<Any>()
                    r == Map::class.java -> emptyMap<Any, Any>()
                    r == Sequence::class.java -> emptySequence<Any>()
                    else -> null
                }
                return KcMissing.miss(what, default)
            }
        }
    }
}

/** A scope over a class's own declarations. */
internal class KcScope(private val decls: List<KaDeclarationSymbol>) : StubKaScope() {
    override val token get() = KcToken
    override val declarations: Sequence<KaDeclarationSymbol> get() = decls.asSequence()
    override val callables: Sequence<KaCallableSymbol> get() = decls.asSequence().filterIsInstance<KaCallableSymbol>()
    override fun callables(nameFilter: (Name) -> Boolean): Sequence<KaCallableSymbol> = callables.filter { c -> nameOf(c)?.let(nameFilter) == true }
    override fun callables(names: Collection<Name>): Sequence<KaCallableSymbol> = callables.filter { nameOf(it) in names }
    override fun callables(vararg names: Name): Sequence<KaCallableSymbol> = callables(names.toList())
    override val classifiers: Sequence<KaClassifierSymbol> get() = decls.asSequence().filterIsInstance<KaClassifierSymbol>()
    override fun classifiers(nameFilter: (Name) -> Boolean): Sequence<KaClassifierSymbol> = classifiers.filter { c -> (c as? KaNamedSymbol)?.name?.let(nameFilter) == true }
    override fun classifiers(names: Collection<Name>): Sequence<KaClassifierSymbol> = classifiers.filter { (it as? KaNamedSymbol)?.name in names }
    override fun classifiers(vararg names: Name): Sequence<KaClassifierSymbol> = classifiers(names.toList())
    override val constructors: Sequence<KaConstructorSymbol> get() = decls.asSequence().filterIsInstance<KaConstructorSymbol>()
    override fun getPackageSymbols(nameFilter: (Name) -> Boolean): Sequence<KaPackageSymbol> = emptySequence()
    override fun getPossibleCallableNames(): Set<Name> = callables.mapNotNull(::nameOf).toSet()
    override fun getPossibleClassifierNames(): Set<Name> = classifiers.mapNotNull { (it as? KaNamedSymbol)?.name }.toSet()
    private fun nameOf(c: KaCallableSymbol): Name? = (c as? KaNamedSymbol)?.name
}

internal class KcScopeContext(override val scopes: List<KaScopeWithKind>) : KaScopeContext {
    override val token get() = KcToken
    override val implicitValues: List<KaScopeImplicitValue> get() = emptyList()
}

internal class KcScopeWithKind(override val scope: KaScope, override val kind: KaScopeKind) : KaScopeWithKind {
    override val token get() = KcToken
}

internal class KcLocalScopeKind(override val indexInTower: Int) : KaScopeKind.LocalScope
internal class KcTypeScopeKind(override val indexInTower: Int) : KaScopeKind.TypeScope
internal class KcExplicitSimpleImportingScopeKind(override val indexInTower: Int) : KaScopeKind.ExplicitSimpleImportingScope
internal class KcPackageMemberScopeKind(override val indexInTower: Int) : KaScopeKind.PackageMemberScope
