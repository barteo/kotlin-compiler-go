@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package io.github.barteo.kc.aa

import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.components.KaScopeContext
import org.jetbrains.kotlin.analysis.api.components.KaScopeImplicitValue
import org.jetbrains.kotlin.analysis.api.components.KaScopeKind
import org.jetbrains.kotlin.analysis.api.components.KaScopeWithKind
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.resolution.*
import org.jetbrains.kotlin.analysis.api.scopes.KaScope
import org.jetbrains.kotlin.analysis.api.signatures.KaFunctionSignature
import org.jetbrains.kotlin.analysis.api.signatures.KaVariableSignature
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaDeclarationContainerSymbol
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.psi.*
import org.jetbrains.kotlin.resolution.KtResolvable
import org.jetbrains.kotlin.types.Variance

/**
 * What a linter asks of the code, answered from a round's facts ([KcLint]):
 * expression types, calls, the declarations names refer to, and scope
 * contexts. [KcAnalysis] dispatches to it.
 */
internal class KcLintAnalysis(private val a: KcAnalysis, private val s: KcLint) {
    private val round get() = a.round

    // Expressions.

    fun expressionType(e: KtExpression): KaType? {
        val target = when (e) {
            is KtParenthesizedExpression -> return e.expression?.let(::expressionType)
            is KtLabeledExpression -> return e.baseExpression?.let(::expressionType)
            is KtAnnotatedExpression -> return e.baseExpression?.let(::expressionType)
            // A call's callee has no type, as in kotlinc, unless it is a
            // value the call invokes: block in block().
            is KtNameReferenceExpression -> {
                val call = (e.parent as? KtCallExpression)?.takeIf { it.calleeExpression == e }
                if (call != null && s.fact(call)?.call?.valueInvocation != true) return null
                // this's name has this's type.
                (e.parent as? KtThisExpression)?.takeIf { it.instanceReference == e } ?: e
            }
            else -> e
        }
        val f = s.fact(target) ?: return null
        val t = f.type ?: return null
        // A type parameter's value that a smart cast makes non-null is
        // T & Any; so is an intersection's, which the facts name as a
        // parameter.
        if (f.flowNonNull && t.parameter) return KcDefinitelyNotNullType(round, s.type(t))
        return s.type(t, nonNull = f.flowNonNull)
    }

    /**
     * The warnings kotlinc reports on [e] that the facts tell: the use of
     * something deprecated, a safe call, !! or elvis on a value that can't
     * be null, a cast that can't succeed and unreachable code. kc reports
     * no warnings itself.
     */
    fun directDiagnostics(e: KtElement): List<org.jetbrains.kotlin.analysis.api.diagnostics.KaDiagnosticWithPsi<*>> {
        if (!s.hasFacts) return emptyList()
        val out = ArrayList<org.jetbrains.kotlin.analysis.api.diagnostics.KaDiagnosticWithPsi<*>>()
        fun add(name: String, message: String) { out += KcLintDiagnostic(e, name, message, round.token) }
        when (e) {
            // A type's name, whose deprecation is its type's.
            is KtSimpleNameExpression -> (s.fact(e)?.deprecated ?: (e.parent as? KtUserType)?.takeIf { it.referenceExpression == e }?.let { s.fact(it)?.deprecated })?.let { message ->
                if (com.intellij.psi.util.PsiTreeUtil.getParentOfType(e, KtImportDirective::class.java) == null && com.intellij.psi.util.PsiTreeUtil.getParentOfType(e, KtPackageDirective::class.java) == null) {
                    add("DEPRECATION", message)
                }
            }
            is KtSafeQualifiedExpression ->
                if (cantBeNull(e.receiverExpression)) add("UNNECESSARY_SAFE_CALL", "Unnecessary safe call on a non-null receiver.")
            is KtPostfixExpression -> if (e.operationToken == KtTokens.EXCLEXCL && e.baseExpression?.let(::cantBeNull) == true) {
                add("UNNECESSARY_NOT_NULL_ASSERTION", "Unnecessary non-null assertion (!!) on a non-null receiver.")
            }
            is KtBinaryExpression -> if (e.operationToken == KtTokens.ELVIS && e.left?.let(::cantBeNull) == true) {
                add("USELESS_ELVIS", "Elvis operator (?:) always returns the left operand of non-nullable type.")
            }
            is KtBinaryExpressionWithTypeRHS -> if (s.fact(e)?.impossibleCast == true) add("CAST_NEVER_SUCCEEDS", "This cast can never succeed.")
            else -> {}
        }
        if (e is KtExpression && s.fact(e)?.unreachable == true) add("UNREACHABLE_CODE", "Unreachable code.")
        return out
    }

    /** Whether [e]'s type, after smart casts, is a non-null type that isn't a platform type or a type parameter. */
    private fun cantBeNull(e: KtExpression): Boolean {
        // A safe call's value is nullable, whatever its receiver.
        if (KtPsiUtil.safeDeparenthesize(e) is KtSafeQualifiedExpression) return false
        val t = expressionType(e) ?: return false
        return t !is KcFlexibleType && t !is KcTypeParameterType && t !is KcErrorType && !a.isMarkedNullable(t)
    }

    /** The operators for loop [e] calls: its iterator(), hasNext() and next(). */
    fun loopOperators(e: KtForExpression): List<KaSymbol> = s.fact(e)?.operatorCalls?.map { s.callableSymbol(it) }.orEmpty()

    fun isUsedAsExpression(e: KtExpression): Boolean =
        s.fact(e)?.usedAsExpression ?: KcMissing.miss("KaSession.isUsedAsExpression without facts", true)

    // Types.

    /** The fully qualified name of [t]'s class, as written or expanded: kotlin.Int. */
    fun classFq(t: KaType): String? = when (val x = round.expand(t)) {
        is KcFlexibleType -> classFq(x.lowerBound)
        is KaClassType -> x.classId.asFqNameString()
        else -> null
    }

    /**
     * Whether [t] is a type of class [fq], nullable or not, as the Analysis
     * API's `isClassType` (and so `isBooleanType`, ...) answers: a flexible
     * type is not a class type.
     */
    fun isClass(t: KaType, fq: String) = (round.expand(t) as? KaClassType)?.classId?.asFqNameString() == fq

    fun isFunctionalInterface(t: KaType): Boolean {
        val c = round.expand(t).let { (it as? KcFlexibleType)?.lowerBound ?: it } as? KaClassType ?: return false
        if (s.isSam(c.classId)) return true
        val sym = c.symbol as? KaNamedClassSymbol ?: return false
        return sym.classKind == KaClassKind.INTERFACE && runCatching { sym.isFun }.getOrDefault(false)
    }

    fun arrayElementType(t: KaType): KaType? = when (val fq = classFq(t)) {
        "kotlin.Array" -> (round.expand(t).let { (it as? KcFlexibleType)?.lowerBound ?: it } as? KaClassType)?.typeArguments?.firstOrNull()?.type
        in primitiveArrays -> s.classType("kotlin/" + fq!!.removePrefix("kotlin.").removeSuffix("Array"), false)
        else -> null
    }

    /** [t]'s supertypes at any depth, nearest first, each class once, as the round's type relations give them. */
    fun allSupertypes(t: KaType): Sequence<KaType> {
        val out = ArrayList<KaType>()
        val seen = HashSet<ClassId>()
        (round.expand(t) as? KaClassType)?.classId?.let(seen::add)
        val queue = ArrayDeque(round.relations.directSupertypes(t))
        while (queue.isNotEmpty()) {
            val st = queue.removeFirst()
            val id = (round.expand(st) as? KaClassType)?.classId
            if (id != null && !seen.add(id)) continue
            out += st
            queue.addAll(round.relations.directSupertypes(st))
        }
        return out.asSequence()
    }

    fun defaultType(c: KaClassifierSymbol): KaType = when (c) {
        is KaClassSymbol -> c.classId?.let { id ->
            KcClassType(round, id, c.typeParameters.map { p ->
                KcTypeArgument(KcTypeParameterType(round, p.name, p, false), Variance.INVARIANT, round.token)
            })
        } ?: KcErrorType(round, "anonymous")
        is KaTypeParameterSymbol -> KcTypeParameterType(round, c.name, c, false)
        else -> KcMissing.miss("KaSession.defaultType of ${c.javaClass.simpleName}", KcErrorType(round, c.toString()))
    }

    // Symbols.

    /** The declaration [sym], a symbol of PSI only kc's facts know, is in. */
    fun containingDeclaration(sym: KaSymbol): KaDeclarationSymbol? {
        if (sym is KcLintReceiverParameterSymbol) return sym.owningCallableSymbol as? KaDeclarationSymbol
        val psi = sym.psi ?: return null
        var p = psi.parent
        while (p != null && p !is KtFile) {
            if (p is KtDeclaration && p !is KtDestructuringDeclaration || p is KtFunctionLiteral) {
                s.declarationSymbol(p)?.let { return it }
            }
            p = p.parent
        }
        return null
    }

    fun isPublicApi(sym: KaDeclarationSymbol): Boolean {
        var cur: KaDeclarationSymbol? = sym
        while (cur != null) {
            val v = when (cur) {
                is KaClassLikeSymbol -> (cur as? KaNamedClassSymbol)?.visibility ?: KaSymbolVisibility.LOCAL
                is KaCallableSymbol -> visibility(cur)
                else -> KaSymbolVisibility.PUBLIC
            }
            // @PublishedApi internal declarations are public API, as inline functions' bodies see them.
            val published = v == KaSymbolVisibility.INTERNAL &&
                (cur.psi as? KtAnnotated)?.annotationEntries?.any { it.shortName?.asString() == "PublishedApi" } == true
            if (v != KaSymbolVisibility.PUBLIC && v != KaSymbolVisibility.PROTECTED && !published) return false
            cur = a.getContainingDeclaration(cur)
        }
        return true
    }

    private fun visibility(d: KaDeclarationSymbol): KaSymbolVisibility = when (d) {
        is KaLocalVariableSymbol, is KaValueParameterSymbol, is KaAnonymousFunctionSymbol -> KaSymbolVisibility.LOCAL
        is KaNamedFunctionSymbol -> d.visibility
        is KaPropertySymbol -> d.visibility
        is KaConstructorSymbol -> d.visibility
        else -> KaSymbolVisibility.PUBLIC
    }

    /** A source class's own declarations, from its PSI: a class only the facts know. */
    fun members(c: KaDeclarationContainerSymbol): List<KaDeclarationSymbol> {
        val cls = (c as? KaSymbol)?.psi as? KtClassOrObject ?: return emptyList()
        val out = ArrayList<KaDeclarationSymbol>()
        cls.primaryConstructor?.let { pc -> s.declarationSymbol(pc)?.let(out::add) }
        for (d in cls.declarations) {
            if (d is KtClassInitializer) continue
            s.declarationSymbol(d)?.let(out::add)
        }
        return out
    }

    private fun memberDecls(c: KaDeclarationContainerSymbol): List<KaDeclarationSymbol> =
        (c as? KcClassSymbol)?.let(round::declared) ?: members(c)

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
                    (s.declarationSymbol(p) as? KaDeclarationContainerSymbol)?.let { decls += memberDecls(it) }
                    p.typeParameters.forEach { t -> s.declarationSymbol(t)?.let(decls::add) }
                    kind = ::KcTypeScopeKind
                }
            }
            if (decls.isNotEmpty()) scopes += KcScopeWithKind(KcScope(decls, round.token), kind(scopes.size))
            child = p
            p = p.parent
        }
        scopes += importingScopes(file, scopes.size)
        return KcScopeContext(scopes, round.token)
    }

    fun importingScopeContext(file: KtFile): KaScopeContext = KcScopeContext(importingScopes(file, 0), round.token)

    private fun importingScopes(file: KtFile, start: Int): List<KaScopeWithKind> {
        val imported = ArrayList<KaDeclarationSymbol>()
        // A star import's package's declarations in the sources: a
        // typealias Runnable in kotlinx.coroutines collides with
        // java.lang.Runnable in a file that imports kotlinx.coroutines.*.
        val starred = ArrayList<KaDeclarationSymbol>()
        for (d in file.importDirectives) {
            if (d.isAllUnder) {
                d.importedFqName?.let { fq -> s.packageDeclarations(fq).mapNotNullTo(starred) { s.declarationSymbol(it) } }
                continue
            }
            val ref = when (val e = d.importedReference) {
                is KtQualifiedExpression -> e.selectorExpression
                else -> e
            } as? KtSimpleNameExpression
            val sym = ref?.let(::referenced) as? KaDeclarationSymbol
                ?: d.importedFqName?.let { fq -> s.sourceClass(ClassId.topLevel(fq))?.let { s.declarationSymbol(it) } ?: namedClass(fq.pathSegments().map { it.asString() }) }
            sym?.let(imported::add)
        }
        val pkg = s.packageDeclarations(file.packageFqName).mapNotNull { s.declarationSymbol(it) }
        return listOf(
            KcScopeWithKind(KcScope(imported, round.token), KcExplicitSimpleImportingScopeKind(start)),
            KcScopeWithKind(KcScope(pkg, round.token), KcPackageMemberScopeKind(start + 1)),
            KcScopeWithKind(KcScope(starred, round.token), KcExplicitStarImportingScopeKind(start + 2)),
        )
    }

    // Calls.

    fun resolveToCall(e: KtElement): KaCallInfo? = call(e)?.let(::KcLintSuccessCallInfo)

    fun resolveSymbol(e: KtResolvable): KaSymbol? = when (e) {
        is KtSimpleNameExpression -> referenced(e)
        is KtQualifiedExpression -> e.selectorExpression?.let { resolveSymbol(it as KtResolvable) }
        is KtElement -> (call(e) as? KaCallableMemberCall<*, *>)?.partiallyAppliedSymbol?.signature?.symbol
        else -> null
    }

    /** The call [e] makes, as the Analysis API's resolveToCall finds it. */
    fun call(e: KtElement): KaCall? {
        KcStats.count("call(element)", e)
        val c = calls[e] ?: (resolveCall(e) ?: noCall).let { calls.putIfAbsent(e, it) ?: it }
        return c as? KaCall
    }

    /**
     * The calls of elements, as [call] finds them, or [noCall]: rules
     * resolve the same element several times, and a KaCall is immutable.
     */
    private val calls = java.util.concurrent.ConcurrentHashMap<KtElement, Any>()
    private val noCall = Any()

    private fun resolveCall(e: KtElement): KaCall? {
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

    private fun receiverValue(e: KtElement, implicit: KcFElem?, passed: KcFType?): KaReceiverValue? {
        explicitReceiver(e)?.let { (expr, safe) ->
            val f = s.fact(expr)
            val declared = f?.type ?: passed
            val explicit = KcLintExplicitReceiver(expr, safe, s.type(declared))
            return if (f?.flowNonNull == true) KcLintSmartCastReceiver(explicit, s.type(declared, nonNull = true)) else explicit
        }
        val psi = s.psi(implicit) ?: return null
        val owner: KaSymbol = when (psi) {
            is KtCallableDeclaration -> (s.declarationSymbol(psi) as? KaCallableSymbol)?.receiverParameter ?: s.declarationSymbol(psi)
            else -> s.declarationSymbol(psi)
        } ?: return null
        val type = passed?.let { s.type(it) } ?: (owner as? KaClassSymbol)?.let(::defaultType) ?: return null
        return KcLintImplicitReceiver(owner, type)
    }

    private fun functionCall(e: KtElement, c: KcFCall?): KaCall? {
        if (c == null) return null
        val sym = s.callableSymbol(c) as? KaFunctionSymbol ?: return null
        val receiver = receiverValue(e, c.implicitReceiver, c.receiver)
        val params = sym.valueParameters
        val paramSigs = params.mapIndexed { i, p ->
            KcLintVariableSignature<KaValueParameterSymbol>(
                p, p.name,
                c.params.getOrNull(i)?.let { if (it.vararg) lintVarargElement(s, it.type) else s.type(it.type) } ?: p.returnType,
            )
        }
        val sig = KcLintFunctionSignature<KaFunctionSymbol>(
            sym, s.type(c.returnType ?: c.declaredReturnType), c.declaredReceiver?.let { s.type(it) }, paramSigs,
        )
        val pas = if (c.extension) KcLintPartiallyApplied<KaFunctionSymbol, KaFunctionSignature<KaFunctionSymbol>>(sig, null, receiver)
        else KcLintPartiallyApplied(sig, receiver, null)
        val mapping = LinkedHashMap<KtExpression, KaVariableSignature<KaValueParameterSymbol>>()
        arguments(e).forEachIndexed { i, arg ->
            val p = c.argumentMapping.getOrNull(i) ?: return@forEachIndexed
            if (arg != null && p >= 0 && p < paramSigs.size) mapping[arg] = paramSigs[p]
        }
        return KcLintSimpleFunctionCall(pas, mapping, c.valueInvocation)
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
            sym != null && (sym.kind == KcFSym.LOCAL || sym.kind == KcFSym.PARAMETER) -> s.symbol(sym) as? KaVariableSymbol
            sym != null && sym.kind == KcFSym.PROPERTY || getter != null ->
                s.propertySymbol(sym?.takeIf { it.kind == KcFSym.PROPERTY }, getter) as? KaVariableSymbol
            else -> null
        } ?: return null
        // The signature has the variable's type; the fact's is the
        // expression's, which a safe call (a?.b) makes nullable.
        val safeSelector = (e.parent as? KtSafeQualifiedExpression)?.selectorExpression == e
        // A Java getter's T! stays T! there.
        val platform = safeSelector && getter?.declaredReturnType?.platform == true
        val type = getter?.returnType?.let { s.type(it) }
            ?: f.type?.takeIf { platform && !it.parameter }?.let {
                s.type(KcFType(it.name, it.classId, it.flags and 2.inv() or 5, it.owner, it.projection, it.args, it.supertypes))
            }
            ?: f.type?.let { s.type(it, nonNull = safeSelector && !it.parameter && !a.isMarkedNullable(vs.returnType)) }
            ?: vs.returnType
        val sig = KcLintVariableSignature<KaVariableSymbol>(vs, vs.name, type)
        val receiver = receiverValue(e, f.implicitReceiver, getter?.receiver)
        val extension = getter?.extension == true
        val pas = KcLintPartiallyApplied<KaVariableSymbol, KaVariableSignature<KaVariableSymbol>>(
            sig, if (extension) null else receiver, if (extension) receiver else null,
        )
        return KcLintVariableAccessCall(pas, accessKind(e))
    }

    private fun accessKind(e: KtSimpleNameExpression): KaVariableAccessCall.Kind {
        var x: KtExpression = e
        (x.parent as? KtQualifiedExpression)?.takeIf { it.selectorExpression == x }?.let { x = it }
        val b = x.parent as? KtBinaryExpression
        return if (b != null && b.operationToken == KtTokens.EQ && b.left == x) KcLintWrite(b.right) else KcLintRead
    }

    /** The declaration a name refers to. */
    fun referenced(e: KtSimpleNameExpression): KaSymbol? {
        KcStats.count("referenced(element)", e)
        val r = references[e] ?: (resolveReference(e) ?: noCall).let { references.putIfAbsent(e, it) ?: it }
        return r as? KaSymbol
    }

    /**
     * The declarations names refer to, as [referenced] finds them, or
     * [noCall]: rules resolve the same name several times, and the
     * session's symbols are shared.
     */
    private val references = java.util.concurrent.ConcurrentHashMap<KtSimpleNameExpression, Any>()

    private fun resolveReference(e: KtSimpleNameExpression): KaSymbol? {
        (e.parent as? KtThisExpression)?.takeIf { it.instanceReference == e }?.let { thisTarget(it)?.let { t -> return t } }
        val f = s.fact(e) ?: return typeName(e) ?: annotationArgument(e)
        f.call?.let { c ->
            // A constructor call's callee names the constructor; a call's, the function.
            if (!c.valueInvocation) return s.callableSymbol(c, s.sym(f.symbol))
        }
        val sym = s.sym(f.symbol)
        if (sym == null) {
            f.accessors.firstOrNull()?.let { return s.propertySymbol(null, it) }
            return null
        }
        if (sym.kind == KcFSym.PROPERTY) return s.propertySymbol(sym, f.accessors.firstOrNull())
        return s.symbol(sym)
    }

    /**
     * What this's reference resolves to: the class or object whose instance
     * it is, or the receiver parameter of the function or lambda.
     */
    private fun thisTarget(e: KtThisExpression): KaSymbol? {
        val target = s.psi(s.fact(e)?.receiverTarget) ?: return null
        return when (target) {
            is KtClassOrObject, is KtObjectLiteralExpression -> s.declarationSymbol(target)
            is KtLambdaExpression -> (s.declarationSymbol(target.functionLiteral) as? KaCallableSymbol)?.receiverParameter
            is KtCallableDeclaration -> (s.declarationSymbol(target) as? KaCallableSymbol)?.receiverParameter
            else -> null
        }
    }

    /**
     * The property a simple name in an annotation's argument refers to,
     * message in @Deprecated(message = message): kc evaluates the
     * arguments, but has no facts on them. The name is looked up in the
     * classes and objects around the annotation, their companions', and
     * the file's top level.
     */
    private fun annotationArgument(e: KtSimpleNameExpression): KaSymbol? {
        if (e !is KtNameReferenceExpression || e.parent is KtQualifiedExpression && (e.parent as KtQualifiedExpression).selectorExpression == e) return null
        val arg = com.intellij.psi.util.PsiTreeUtil.getParentOfType(e, KtValueArgument::class.java) ?: return null
        if (com.intellij.psi.util.PsiTreeUtil.getParentOfType(arg, KtAnnotationEntry::class.java) == null) return null
        if (arg.getArgumentName()?.referenceExpression == e) return null
        val name = e.getReferencedName()
        fun find(ds: List<KtDeclaration>): KtProperty? = ds.firstOrNull { it is KtProperty && it.name == name } as KtProperty?
        var c = com.intellij.psi.util.PsiTreeUtil.getParentOfType(e, KtClassOrObject::class.java)
        while (c != null) {
            val p = find(c.declarations) ?: (c as? KtClass)?.companionObjects?.firstNotNullOfOrNull { find(it.declarations) }
            if (p != null) return s.declarationSymbol(p)
            c = com.intellij.psi.util.PsiTreeUtil.getParentOfType(c, KtClassOrObject::class.java)
        }
        return find(e.containingKtFile.declarations)?.let(s::declarationSymbol)
    }

    /**
     * The class the name of a type refers to: kc's facts are on the
     * USER_TYPE, not on its name. A qualified name refers to the class
     * written, as in kotlinc: java.lang.annotation.Annotation is the Java
     * interface, not kotlin.Annotation, the type kc maps it to.
     */
    private fun typeName(e: KtSimpleNameExpression): KaSymbol? {
        val u = e.parent as? KtUserType ?: return null
        if (u.referenceExpression != e) return null
        val f = s.fact(u) ?: return null
        val t = f.type ?: return null
        if (u.qualifier != null) writtenClass(u)?.let { return it }
        // A typealias names its expansion's class: the alias, as the fact
        // names it for one in the sources, or as an import of the name does.
        f.importableName?.takeIf { it.isNotEmpty() }?.let { n -> round.typeAlias(ClassId.topLevel(FqName(n)))?.let { return it } }
        val id = t.classId.takeIf { it.isNotEmpty() } ?: return null
        val classId = runCatching { ClassId.fromString(id) }.getOrNull() ?: return null
        importedAlias(e.containingKtFile, e.getReferencedName(), classId)?.let { return it }
        return round.classes[classId] ?: round.libraryClass(classId)
    }

    /** The typealias an explicit import names [name] that expands to class [classId], as kotlin.concurrent.Volatile to kotlin.jvm.Volatile. */
    private fun importedAlias(file: KtFile, name: String, classId: ClassId): KaSymbol? {
        for (d in file.importDirectives) {
            if (d.isAllUnder || d.importedName?.asString() != name) continue
            val fq = d.importedFqName ?: continue
            val alias = round.typeAlias(ClassId.topLevel(fq)) ?: continue
            if ((alias.expandedType as? org.jetbrains.kotlin.analysis.api.types.KaClassType)?.classId == classId) return alias
        }
        return null
    }

    /** The class a qualified type's name spells, a.b.C.D: package a.b and class C.D, or a.b.C and D, and so on. */
    private fun writtenClass(u: KtUserType): KaSymbol? {
        val parts = generateSequence(u) { it.qualifier }.map { it.referencedName }.toList()
        if (parts.any { it == null }) return null
        return namedClassLike(parts.filterNotNull().asReversed())
    }

    /** The class a fully qualified name names: package a.b and class C.D, or a.b.C and D, and so on. */
    private fun namedClass(names: List<String>): KaClassSymbol? {
        for (i in names.size - 1 downTo 1) {
            val id = ClassId(FqName.fromSegments(names.subList(0, i)), FqName.fromSegments(names.subList(i, names.size)), false)
            (round.classes[id] ?: round.libraryClass(id))?.let { return it }
        }
        return null
    }

    /** The class or top-level typealias a fully qualified name names. */
    private fun namedClassLike(names: List<String>): KaSymbol? =
        namedClass(names) ?: names.takeIf { it.size > 1 }?.let { round.typeAlias(ClassId.topLevel(FqName.fromSegments(it))) }

    companion object {
        val primitives = setOf("kotlin.Int", "kotlin.Long", "kotlin.Short", "kotlin.Byte", "kotlin.Char", "kotlin.Boolean", "kotlin.Float", "kotlin.Double")
        val primitiveArrays = primitives.map { it + "Array" }.toSet()
        val functionClass = Regex("""kotlin\.Function\d+""")
    }
}

internal class KcScopeContext(override val scopes: List<KaScopeWithKind>, override val token: KaLifetimeToken) : KaScopeContext {
    override val implicitValues: List<KaScopeImplicitValue> get() = emptyList()
}

internal class KcScopeWithKind(override val scope: KaScope, override val kind: KaScopeKind) : KaScopeWithKind {
    override val token get() = scope.token
}

internal class KcLocalScopeKind(override val indexInTower: Int) : KaScopeKind.LocalScope
internal class KcTypeScopeKind(override val indexInTower: Int) : KaScopeKind.TypeScope
internal class KcExplicitSimpleImportingScopeKind(override val indexInTower: Int) : KaScopeKind.ExplicitSimpleImportingScope
internal class KcPackageMemberScopeKind(override val indexInTower: Int) : KaScopeKind.PackageMemberScope
internal class KcExplicitStarImportingScopeKind(override val indexInTower: Int) : KaScopeKind.ExplicitStarImportingScope

/**
 * The compiler warnings a linter reads ([KcLintAnalysis.directDiagnostics]),
 * told by their factory names, as kotlinc's. kc's DEPRECATION has the
 * deprecation's message as its default message.
 */
internal class KcLintDiagnostic(
    override val psi: PsiElement,
    override val factoryName: String,
    override val defaultMessage: String,
    override val token: KaLifetimeToken,
) : org.jetbrains.kotlin.analysis.api.diagnostics.KaDiagnosticWithPsi<PsiElement> {
    override val textRanges: Collection<com.intellij.openapi.util.TextRange> get() = listOf(psi.textRange)
    override val diagnosticClass get() = KcLintDiagnostic::class
    override val severity get() = org.jetbrains.kotlin.analysis.api.diagnostics.KaSeverity.WARNING
}
