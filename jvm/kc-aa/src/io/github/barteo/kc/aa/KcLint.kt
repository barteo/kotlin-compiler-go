package io.github.barteo.kc.aa

import com.intellij.lang.ASTNode
import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.symbols.KaCallableSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaClassLikeSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaDeclarationSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaKotlinPropertySymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaTypeParameterSymbol
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.types.KaTypeProjection
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.*
import org.jetbrains.kotlin.types.Variance
import java.util.IdentityHashMap
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CompletionException
import java.util.concurrent.ConcurrentHashMap

/**
 * The facts of a round's sources' code, for a linter (detekt): what kc
 * resolved in their bodies (libkc's OpKaFacts), keyed by psi node, read
 * file by file as they are first asked for. Their types become the round's
 * types ([KcClassType]...), and the declarations they name the round's
 * symbols: a source declaration's model symbol ([KcRound.byPsi]) where the
 * model has one, a library class's model class. What the model has no
 * symbol for, a local declaration in a nested block, a lambda, an
 * anonymous object, or a library callee kc resolved a call to, gets a
 * symbol of its own here ([KcLintSymbols.kt]), made from the facts and
 * the PSI.
 */
internal class KcLint(
    val round: KcRound,
    /** The model's Kotlin files, in the model's order, which the facts' file indexes count. */
    private val files: List<Pair<KcFile, KtFile>>,
    /**
     * Each file's facts, being decoded in the background, if the model has
     * facts: the sources have no errors.
     */
    private val prefetched: List<CompletableFuture<KcFileFacts>>?,
) {
    /** Whether the model has facts: the sources have no errors. */
    val hasFacts: Boolean get() = prefetched != null

    val token get() = round.token
    val noAnnotations: KaAnnotationList by lazy { KcAnnotationList(emptyList(), round.token) }

    private val indexOf = IdentityHashMap<KtFile, Int>().also { m -> files.forEachIndexed { i, (_, f) -> m[f] = i } }

    /** A file's facts and the join of its PSI with kc's psi nodes. */
    inner class FileData(val nodes: Array<ASTNode>, val ids: IdentityHashMap<ASTNode, Int>, val facts: KcFileFacts?)

    private val data = arrayOfNulls<FileData>(files.size)
    private val locks = Array(files.size) { Any() }

    /** The symbols the facts read so far name, by ID. */
    private val syms = ConcurrentHashMap<Int, KcFSym>()

    fun data(file: KtFile): FileData? = indexOf[file]?.let(::data)

    fun data(i: Int): FileData? {
        data[i]?.let { return it }
        synchronized(locks[i]) {
            data[i]?.let { return it }
            val (kf, file) = files[i]
            val nodes = KcRound.preorder(file.node)
            val ids = IdentityHashMap<ASTNode, Int>(nodes.size * 2)
            for (n in 1 until nodes.size) ids[nodes[n]] = n
            var facts: KcFileFacts? = null
            if (prefetched != null) {
                if (kf.nodes != nodes.size - 1 || kf.kinds != KcKinds.hash(nodes)) {
                    System.err.println("kc: the PSI of ${kf.path} differs from kc's tree (${kf.nodes} nodes in kc, ${nodes.size - 1} in PSI), so it has no types")
                } else {
                    facts = try {
                        prefetched[i].join()
                    } catch (e: CompletionException) {
                        throw e.cause ?: e
                    }
                    facts.symbols.forEach { syms.putIfAbsent(it.id, it) }
                }
            }
            return FileData(nodes, ids, facts).also { data[i] = it }
        }
    }

    /** The fact of [e]'s node, if kc has one. */
    fun fact(e: PsiElement?): KcFact? {
        KcStats.count("fact(element)", e)
        if (e == null) return null
        val file = e.containingFile as? KtFile ?: return null
        val d = data(file) ?: return null
        val id = d.ids[e.node] ?: return null
        return d.facts?.get(id)
    }

    fun psi(el: KcFElem?): PsiElement? {
        if (el == null) return null
        val d = data(el.file) ?: return null
        return d.nodes.getOrNull(el.node)?.psi
    }

    fun sym(id: Int): KcFSym? = if (id == 0) null else syms[id]

    // Types.

    val any: KaType by lazy { classType("kotlin/Any", false) }
    val unit: KaType by lazy { classType("kotlin/Unit", false) }

    fun classType(classId: String, nullable: Boolean, args: List<KaTypeProjection> = emptyList()): KcClassType =
        KcClassType(round, ClassId.fromString(classId), args, nullable)

    fun errorType(text: String): KaType = KcErrorType(round, text)

    /** The classes the facts say are functional interfaces (SAM). */
    private val samClasses: MutableSet<ClassId> = ConcurrentHashMap.newKeySet()

    fun isSam(id: ClassId) = id in samClasses

    /** The KaType of [t]; [nonNull] drops its nullability, as a smart cast does. */
    fun type(t: KcFType?, nonNull: Boolean = false): KaType {
        KcStats.count("type(KcFType)", t)
        if (t == null) return KcErrorType(round, "unresolved")
        val cache = if (nonNull) nonNullTypes else types
        cache[t]?.let { return it }
        val made = makeType(t, nonNull)
        return cache.putIfAbsent(t, made) ?: made
    }

    /**
     * The KaTypes made of the facts' types, by type, for [type] without and
     * with nonNull. A file's types are interned, and rules ask for the same
     * few thousand hundreds of thousands of times. KaTypes are immutable,
     * so they are shared, along with what they compute lazily.
     */
    private val types = ConcurrentHashMap<KcFType, KaType>()
    private val nonNullTypes = ConcurrentHashMap<KcFType, KaType>()

    private fun makeType(t: KcFType, nonNull: Boolean): KaType {
        if (t.name.isEmpty()) return KcErrorType(round, "unresolved")
        if (t.parameter && t.owner == null && " & " in t.name) {
            // An intersection, A & B, which the facts name as a parameter:
            // a value of it isn't null, unless it is A? & B?.
            val p = KcTypeParameterType(round, Name.identifier(t.name), typeParameter(t), false)
            return if (t.markedNullable && !nonNull) p.withNullability(true) else KcDefinitelyNotNullType(round, p)
        }
        if (t.parameter) {
            val p = KcTypeParameterType(round, Name.identifier(t.name), typeParameter(t), t.markedNullable && !nonNull)
            return if (t.platform && !nonNull) KcFlexibleType(round, p.withNullability(false), p.withNullability(true)) else p
        }
        if (t.classId.isEmpty()) return KcErrorType(round, t.name)
        if (t.platform && !nonNull) {
            return KcFlexibleType(round, classTypeOf(t, false), classTypeOf(t, true))
        }
        return classTypeOf(t, t.markedNullable && !nonNull)
    }

    private fun classTypeOf(t: KcFType, nullable: Boolean): KaType {
        val args = t.args.map { a ->
            if (a.star) KcStarProjection(round.token) else KcTypeArgument(type(a), variance(a.projection), round.token)
        }
        val id = runCatching { ClassId.fromString(t.classId) }.getOrElse { ClassId.topLevel(FqName(t.name)) }
        if (t.sam) samClasses += id
        return if (KcTypeContext.isFunctionClass(t.classId)) KcFunctionType(round, id, args, nullable, t.receiver, 0)
        // An object expression's type: its class is the expression's.
        else if (t.owner != null) KcClassType(round, id, args, nullable, local = declarationSymbol(psi(t.owner)) as? KaClassLikeSymbol, localPsi = psi(t.owner))
        else KcClassType(round, id, args, nullable)
    }

    private fun typeParameter(t: KcFType): KaTypeParameterSymbol {
        val owner = psi(t.owner) as? KtTypeParameterListOwner
        val p = owner?.typeParameters?.firstOrNull { it.name == t.name }
        return declarationSymbol(p) as? KaTypeParameterSymbol ?: KcLibraryTypeParameterSymbol(round, Name.identifier(t.name))
    }

    private fun variance(p: Int) = when (p) {
        1 -> Variance.IN_VARIANCE
        2 -> Variance.OUT_VARIANCE
        else -> Variance.INVARIANT
    }

    // Symbols.

    private val bySource = ConcurrentHashMap<PsiElement, KaSymbol>()
    private val byKey = ConcurrentHashMap<String, KaSymbol>()

    /** The model's symbol of a source declaration, if it has one. */
    private fun modelSymbol(e: PsiElement): KaDeclarationSymbol? =
        round.byPsi[e] as? KaDeclarationSymbol

    /**
     * The symbol of a declaration in the sources: the model's, or else one
     * made from its PSI and facts.
     */
    fun declarationSymbol(e: PsiElement?): KaDeclarationSymbol? {
        if (e == null) return null
        modelSymbol(e)?.let { return it }
        bySource[e]?.let { return it as? KaDeclarationSymbol }
        val s: KaSymbol = when (e) {
            is KtClassOrObject ->
                if (e is KtObjectDeclaration && e.isObjectLiteral()) KcLintAnonymousObjectSymbol(this, e) else KcLintClassSymbol(this, e)
            is KtObjectLiteralExpression -> return declarationSymbol(e.objectDeclaration)
            is KtNamedFunction ->
                if (e.nameIdentifier == null && e.isLocal) KcLintAnonymousFunctionSymbol(this, e) else KcLintFunctionSymbol(this, e)
            is KtFunctionLiteral -> KcLintAnonymousFunctionSymbol(this, e)
            is KtLambdaExpression -> return declarationSymbol(e.functionLiteral)
            is KtProperty -> if (e.isLocal) KcLintLocalVariableSymbol(this, e) else KcLintPropertySymbol(this, e)
            // A for loop's variable and a catch clause's parameter are local variables, as in FIR.
            is KtParameter ->
                if (e.isLoopParameter || e.parent?.parent is KtCatchClause) KcLintLocalVariableSymbol(this, e) else KcLintValueParameterSymbol(this, e)
            is KtDestructuringDeclarationEntry -> KcLintLocalVariableSymbol(this, e)
            is KtDestructuringDeclaration -> KcLintDestructuringSymbol(this, e)
            is KtConstructor<*> -> KcLintConstructorSymbol(this, e)
            is KtTypeParameter -> KcLintTypeParameterSymbol(this, e)
            is KtTypeAlias -> KcLintTypeAliasSymbol(this, e)
            is KtPropertyAccessor -> {
                return when (val p = declarationSymbol(e.property)) {
                    is KcLintPropertySymbol -> if (e.isGetter) p.getter else p.setter
                    is KaKotlinPropertySymbol -> if (e.isGetter) p.getter else p.setter
                    else -> null
                }
            }
            else -> return null
        }
        return (bySource.putIfAbsent(e, s) ?: s) as? KaDeclarationSymbol
    }

    /** The symbol kc's symbol [sym] stands for. */
    fun symbol(sym: KcFSym?): KaSymbol? {
        if (sym == null) return null
        psi(sym.declaration)?.let { decl -> declarationSymbol(decl)?.let { return it } }
        return when (sym.kind) {
            KcFSym.CLASS -> classSymbol(binaryClassId(sym.name))
            // A lambda's implicit it, which has no declaration.
            KcFSym.PARAMETER -> if (sym.name == "it") KcLintImplicitParameterSymbol(this, null, sym) else null
            KcFSym.PROPERTY -> keyed("p:${sym.qualifiedName}:${sym.owner}") { KcLintLibraryPropertySymbol(this, sym, null) }
            KcFSym.FUNCTION, KcFSym.CONSTRUCTOR ->
                keyed("f:${sym.qualifiedName}:${sym.owner}:${sym.type}") { KcLintLibraryFunctionSymbol(this, null, sym) }
            else -> null
        }
    }

    /** The symbol of a callee [call] calls. */
    fun callableSymbol(call: KcFCall, sym: KcFSym? = null): KaCallableSymbol {
        psi(call.declaration)?.let { decl ->
            (declarationSymbol(decl) as? KaCallableSymbol)?.let { return it }
            if (decl is KtClassOrObject && call.isConstructor) {
                // A class without a written primary constructor: the model's
                // implicit one, if it has it.
                (declarationSymbol(decl) as? KcClassSymbol)?.let { c ->
                    round.declared(c).firstOrNull { it is KcConstructorSymbol && it.isPrimary }?.let { return it as KaCallableSymbol }
                }
                return keyed("c:${call.name}:${call.params.size}") { KcLintLibraryConstructorSymbol(this, call, decl) }
            }
        }
        val key = buildString {
            append(call.name).append('(')
            call.params.forEach { append(it.type).append(',') }
            append(')').append(call.declaredReceiver ?: "")
        }
        return keyed(key) {
            if (call.isConstructor) KcLintLibraryConstructorSymbol(this, call, null) else KcLintLibraryFunctionSymbol(this, call, sym)
        }
    }

    /** The symbol of a property read through accessor [getter], named [sym]. */
    fun propertySymbol(sym: KcFSym?, getter: KcFCall?): KaCallableSymbol? {
        psi(sym?.declaration)?.let { decl -> (declarationSymbol(decl) as? KaCallableSymbol)?.let { return it } }
        psi(getter?.declaration)?.let { decl -> (declarationSymbol(decl) as? KaCallableSymbol)?.let { return it } }
        if (sym == null && getter == null) return null
        val name = sym?.qualifiedName?.takeIf { it.isNotEmpty() } ?: getter?.name ?: sym!!.name
        return keyed("p:$name:${sym?.owner}") { KcLintLibraryPropertySymbol(this, sym, getter) }
    }

    /** The symbol of the class [id] names: the round's. */
    fun classSymbol(id: ClassId): KaClassLikeSymbol = round.classSymbol(id)

    /** The source class [id] names, if the sources have it. */
    fun sourceClass(id: ClassId): KtClassOrObject? = round.classes[id]?.psi as? KtClassOrObject

    /** The top-level declarations of the sources in package [pkg]. */
    fun packageDeclarations(pkg: FqName): List<KtDeclaration> = byPackage[pkg].orEmpty()

    private val byPackage: Map<FqName, List<KtDeclaration>> by lazy {
        files.map { it.second }.groupBy { it.packageFqName }.mapValues { (_, fs) -> fs.flatMap { it.declarations } }
    }

    @Suppress("UNCHECKED_CAST")
    private fun <T : KaSymbol> keyed(key: String, make: () -> T): T =
        (byKey[key] ?: make().let { byKey.putIfAbsent(key, it) ?: it }) as T

    companion object {
        /** The class id of a JVM binary name, a/b/C$D. */
        fun binaryClassId(name: String): ClassId {
            val i = name.lastIndexOf('/')
            val pkg = if (i < 0) "" else name.substring(0, i).replace('/', '.')
            return ClassId(FqName(pkg), FqName(name.substring(i + 1).replace('$', '.')), false)
        }

        /** The class id of a dotted fully qualified name, as the conventions of class names split it. */
        fun fqClassId(fq: String): ClassId {
            val parts = fq.split('.')
            val first = parts.indexOfFirst { it.isNotEmpty() && it[0].isUpperCase() }.let { if (it < 0) parts.size - 1 else it }
            return ClassId(FqName.fromSegments(parts.subList(0, first)), FqName.fromSegments(parts.subList(first, parts.size)), false)
        }
    }
}
