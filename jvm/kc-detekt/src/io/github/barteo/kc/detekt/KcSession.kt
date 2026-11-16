package io.github.barteo.kc.detekt

import com.intellij.lang.ASTNode
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.symbols.KaCallableSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaClassLikeSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaDeclarationSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbol
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.types.KaTypeProjection
import org.jetbrains.kotlin.kdoc.lexer.KDocTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.psi.*
import org.jetbrains.kotlin.types.Variance
import java.util.IdentityHashMap
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import kotlin.math.max
import kotlin.math.min

/**
 * A session over a module's files: kc analyzes them, in the background, and
 * the session maps detekt's PSI to kc's facts by psi node, and kc's facts
 * to Analysis API types and symbols.
 */
internal class KcSession(val project: Project, files: List<KtFile>, private val cfg: KcLintOps.Config) {
    /** The files kc analyzes: the module's Kotlin files, without scripts. */
    private val analyzed: List<KtFile> = files.filter { !it.isScript() && it.virtualFile != null }
    private val indexOf = IdentityHashMap<KtFile, Int>().also { m -> analyzed.forEachIndexed { i, f -> m[f] = i } }

    private class Analysis(val result: KcLintOps.Result?, val ok: Boolean)

    private val paths = analyzed.map { it.virtualFile.path }

    private fun analyze(): Analysis {
        val start = System.nanoTime()
        val r = try {
            KcLintOps.analyze(cfg, paths)
        } catch (e: Throwable) {
            System.err.println("kc: analysis failed, so detekt's type-resolution rules see no types: $e")
            return Analysis(null, false)
        }
        KcDetekt.log("analyzed ${analyzed.size} files in ${(System.nanoTime() - start) / 1_000_000} ms")
        if (r.diagnostics.isNotEmpty()) {
            System.err.println("kc: kc can't compile the sources (${r.diagnostics.size} errors), so detekt's type-resolution rules see no types:")
            r.diagnostics.take(10).forEach { System.err.println("kc:   $it") }
        }
        return Analysis(r, r.diagnostics.isEmpty())
    }

    /**
     * kc's analysis and each file's facts. Unless -Dkc.eager=false, they
     * are computed in the background from the moment the session is
     * built: kc reads the sources from disk, not detekt's PSI, so it
     * overlaps detekt's own setup and its first rules, and the facts are
     * decoded off the thread that runs the rules. Only the join of a
     * file's facts with its PSI waits for that file.
     */
    private val pool: ExecutorService? = if (!eager) null else Executors.newFixedThreadPool(
        max(1, min(cfg.jobs, 4)),
    ) { r -> Thread(r, "kc-facts").apply { isDaemon = true } }

    private val analysisFuture: CompletableFuture<Analysis>? = pool?.let { CompletableFuture.supplyAsync(::analyze, it) }

    private val prefetched: List<CompletableFuture<KcFileFacts?>>? = analysisFuture?.let { a ->
        List(analyzed.size) { i ->
            a.thenApplyAsync({ an -> an.result?.takeIf { an.ok }?.let { KcLintOps.file(it.handle, i, it.files[i].nodes) } }, pool!!)
        }.also { all -> CompletableFuture.allOf(*all.toTypedArray()).whenComplete { _, _ -> pool!!.shutdown() } }
    }

    private val lazyAnalysis: Analysis by lazy(LazyThreadSafetyMode.SYNCHRONIZED) { analyze() }

    private val analysis: Analysis get() = analysisFuture?.join() ?: lazyAnalysis

    private fun factsOf(result: KcLintOps.Result, i: Int): KcFileFacts {
        val f = prefetched?.get(i) ?: return KcLintOps.file(result.handle, i, result.files[i].nodes)
        return try {
            f.join()!!
        } catch (e: java.util.concurrent.CompletionException) {
            throw e.cause ?: e
        }
    }

    /** A file's facts and the join of its PSI with kc's psi nodes. */
    inner class FileData(val index: Int, val nodes: Array<ASTNode>, val ids: IdentityHashMap<ASTNode, Int>, val facts: KcFileFacts?)

    private val data = arrayOfNulls<FileData>(analyzed.size)
    private val locks = Array(analyzed.size) { Any() }

    /** The symbols the facts read so far name, by ID. */
    private val syms = ConcurrentHashMap<Int, KcSym>()

    fun data(file: KtFile): FileData? {
        val i = indexOf[file] ?: return null
        return data(i)
    }

    fun data(i: Int): FileData? {
        data[i]?.let { return it }
        synchronized(locks[i]) {
            data[i]?.let { return it }
            val a = analysis
            val file = analyzed[i]
            val nodes = preorder(file.node)
            val ids = IdentityHashMap<ASTNode, Int>(nodes.size * 2)
            for (n in 1 until nodes.size) ids[nodes[n]] = n
            var facts: KcFileFacts? = null
            val result = a.result
            if (a.ok && result != null) {
                val shape = result.files[i]
                if (shape.nodes != nodes.size - 1 || shape.kinds != KcKinds.hash(nodes)) {
                    System.err.println("kc: the PSI of ${file.virtualFile.path} differs from kc's tree (${shape.nodes} nodes in kc, ${nodes.size - 1} in PSI), so it has no types")
                } else {
                    facts = factsOf(result, i)
                    facts.symbols.forEach { syms.putIfAbsent(it.id, it) }
                }
            }
            return FileData(i, nodes, ids, facts).also { data[i] = it }
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

    fun psi(el: KcElem?): PsiElement? {
        if (el == null) return null
        val d = data(el.file) ?: return null
        return d.nodes.getOrNull(el.node)?.psi
    }

    fun sym(id: Int): KcSym? = if (id == 0) null else syms[id]

    /** Whether kc's facts are there: the sources compiled. */
    val hasFacts: Boolean get() = analysis.ok

    // Types.

    val any: KaType by lazy { classType("kotlin/Any", false) }
    val unit: KaType by lazy { classType("kotlin/Unit", false) }
    val nothing: KaType by lazy { classType("kotlin/Nothing", false) }

    fun classType(classId: String, nullable: Boolean, args: List<KaTypeProjection> = emptyList()): KcClassType =
        KcClassType(this, ClassId.fromString(classId), args, nullable, null)

    /** The KaType of [t]; [nonNull] drops its nullability, as a smart cast does. */
    fun type(t: KcType?, nonNull: Boolean = false): KaType {
        KcStats.count("type(KcType)", t)
        if (t == null) return KcErrorType(this, "unresolved")
        if (!cacheTypes) return makeType(t, nonNull)
        val cache = if (nonNull) nonNullTypes else types
        cache[t]?.let { return it }
        val made = makeType(t, nonNull)
        return cache.putIfAbsent(t, made) ?: made
    }

    /**
     * The KaTypes made of kc's types, by type, for [type] without and with
     * nonNull. A file's types are interned, and rules ask for the same few
     * thousand hundreds of thousands of times. KaTypes are immutable, so
     * they are shared, along with what they compute lazily, such as their
     * supertypes' names for subtyping.
     */
    private val types = ConcurrentHashMap<KcType, KaType>()
    private val nonNullTypes = ConcurrentHashMap<KcType, KaType>()

    private fun makeType(t: KcType, nonNull: Boolean): KaType {
        if (t.name.isEmpty()) return KcErrorType(this, "unresolved")
        if (t.parameter) {
            val p = KcTypeParameterType(this, t, (t.markedNullable) && !nonNull)
            return if (t.platform && !nonNull) KcFlexibleType(this, p.withNullability(false), p.withNullability(true)) else p
        }
        if (t.classId.isEmpty()) return KcErrorType(this, t.name)
        if (t.platform && !nonNull) {
            return KcFlexibleType(this, classTypeOf(t, false), classTypeOf(t, true))
        }
        return classTypeOf(t, t.markedNullable && !nonNull)
    }

    private fun classTypeOf(t: KcType, nullable: Boolean): KaType {
        val args = t.args.map { a ->
            if (a.star) KcStarProjection(this) else KcTypeArgument(this, type(a), variance(a.projection))
        }
        val id = runCatching { ClassId.fromString(t.classId) }.getOrElse { ClassId.topLevel(FqName(t.name)) }
        return if (functionClass.matches(t.classId)) KcFunctionType(this, id, args, nullable, t)
        else KcClassType(this, id, args, nullable, t)
    }

    private fun variance(p: Int) = when (p) {
        1 -> Variance.IN_VARIANCE
        2 -> Variance.OUT_VARIANCE
        else -> Variance.INVARIANT
    }

    // Symbols.

    private val bySource = ConcurrentHashMap<PsiElement, KaSymbol>()
    private val byKey = ConcurrentHashMap<String, KaSymbol>()

    /** The symbol of a declaration in the sources. */
    fun declarationSymbol(e: PsiElement?): KaDeclarationSymbol? {
        if (e == null) return null
        bySource[e]?.let { return it as? KaDeclarationSymbol }
        val s: KaSymbol = when (e) {
            is KtClassOrObject -> if (e is KtObjectDeclaration && e.isObjectLiteral()) KcAnonymousObjectSymbol(this, e) else KcSourceClassSymbol(this, e)
            is KtObjectLiteralExpression -> return declarationSymbol(e.objectDeclaration)
            is KtNamedFunction -> if (e.nameIdentifier == null && e.isLocal) KcAnonymousFunctionSymbol(this, e) else KcSourceFunctionSymbol(this, e)
            is KtFunctionLiteral -> KcAnonymousFunctionSymbol(this, e)
            is KtLambdaExpression -> return declarationSymbol(e.functionLiteral)
            is KtProperty -> if (e.isLocal) KcLocalVariableSymbol(this, e) else KcSourcePropertySymbol(this, e)
            // A for loop's variable and a catch clause's parameter are local variables, as in FIR.
            is KtParameter -> if (e.isLoopParameter || e.parent?.parent is KtCatchClause) KcLocalVariableSymbol(this, e) else KcValueParameterSymbol(this, e)
            is KtDestructuringDeclarationEntry -> KcLocalVariableSymbol(this, e)
            is KtDestructuringDeclaration -> KcDestructuringSymbol(this, e)
            is KtConstructor<*> -> KcSourceConstructorSymbol(this, e)
            is KtTypeParameter -> KcTypeParameterSymbol(this, e)
            is KtTypeAlias -> KcTypeAliasSymbol(this, e)
            is KtPropertyAccessor -> {
                val p = declarationSymbol(e.property) as? KcSourcePropertySymbol ?: return null
                return if (e.isGetter) p.getter else p.setter
            }
            else -> return null
        }
        return (bySource.putIfAbsent(e, s) ?: s) as? KaDeclarationSymbol
    }

    /** The symbol kc's symbol [sym] stands for. */
    fun symbol(sym: KcSym?): KaSymbol? {
        if (sym == null) return null
        psi(sym.declaration)?.let { decl ->
            declarationSymbol(decl)?.let { return it }
            // A primary constructor's parameter that declares a property.
            if (decl is KtParameter) return declarationSymbol(decl)
        }
        return when (sym.kind) {
            KcSym.CLASS -> classSymbol(binaryClassId(sym.name))
            // A lambda's implicit it, which has no declaration.
            KcSym.PARAMETER -> if (sym.name == "it") KcImplicitParameterSymbol(this, null, sym) else null
            KcSym.PROPERTY -> keyed("p:${sym.qualifiedName}:${sym.owner}") { KcLibraryPropertySymbol(this, sym, null) }
            KcSym.FUNCTION, KcSym.CONSTRUCTOR -> keyed("f:${sym.qualifiedName}:${sym.owner}:${sym.type}") { KcLibraryFunctionSymbol(this, null, sym) }
            else -> null
        }
    }

    /** The symbol of a callee [call] calls. */
    fun callableSymbol(call: KcCall, sym: KcSym? = null): KaCallableSymbol {
        psi(call.declaration)?.let { decl ->
            (declarationSymbol(decl) as? KaCallableSymbol)?.let { return it }
            if (decl is KtClassOrObject && call.isConstructor) {
                // A class without a written primary constructor.
                return keyed("c:${call.name}:${call.params.size}") { KcLibraryConstructorSymbol(this, call, decl) }
            }
        }
        val key = buildString {
            append(call.name).append('(')
            call.params.forEach { append(it.type).append(',') }
            append(')').append(call.declaredReceiver ?: "")
        }
        return keyed(key) {
            if (call.isConstructor) KcLibraryConstructorSymbol(this, call, null) else KcLibraryFunctionSymbol(this, call, sym)
        }
    }

    /** The symbol of a property read through accessor [getter], named [sym]. */
    fun propertySymbol(sym: KcSym?, getter: KcCall?): KaCallableSymbol? {
        psi(sym?.declaration)?.let { decl -> (declarationSymbol(decl) as? KaCallableSymbol)?.let { return it } }
        psi(getter?.declaration)?.let { decl -> (declarationSymbol(decl) as? KaCallableSymbol)?.let { return it } }
        if (sym == null && getter == null) return null
        val name = sym?.qualifiedName?.takeIf { it.isNotEmpty() } ?: getter?.name ?: sym!!.name
        return keyed("p:$name:${sym?.owner}") { KcLibraryPropertySymbol(this, sym, getter) }
    }

    private val classes = ConcurrentHashMap<ClassId, KaClassLikeSymbol>()

    /** The symbol of the class [id] names: its declaration's, if it is in the sources. */
    fun classSymbol(id: ClassId): KaClassLikeSymbol = classes.getOrPut(id) {
        sourceClasses[id]?.let { declarationSymbol(it) as? KaClassLikeSymbol } ?: KcLibraryClassSymbol(this, id)
    }

    /** The classes of the sources, by class id. */
    private val sourceClasses: Map<ClassId, KtClassOrObject> by lazy {
        val m = HashMap<ClassId, KtClassOrObject>()
        for (f in analyzed) {
            f.accept(object : KtTreeVisitorVoid() {
                override fun visitClassOrObject(c: KtClassOrObject) {
                    c.getClassId()?.let { m[it] = c }
                    super.visitClassOrObject(c)
                }
            })
        }
        m
    }

    /** The top-level declarations of the sources in package [pkg]. */
    fun packageDeclarations(pkg: FqName): List<KtDeclaration> = byPackage[pkg].orEmpty()

    private val byPackage: Map<FqName, List<KtDeclaration>> by lazy {
        analyzed.groupBy { it.packageFqName }.mapValues { (_, fs) -> fs.flatMap { it.declarations } }
    }

    /** The supertypes kc lists for the source class [id], if it has one. */
    fun sourceClass(id: ClassId): KtClassOrObject? = sourceClasses[id]

    @Suppress("UNCHECKED_CAST")
    private fun <T : KaSymbol> keyed(key: String, make: () -> T): T = (byKey[key] ?: make().let { byKey.putIfAbsent(key, it) ?: it }) as T

    companion object {
        /** Whether kc analyzes in the background as the session is built: -Dkc.eager, true by default. */
        private val eager = System.getProperty("kc.eager") != "false"

        /** Whether the session shares the KaTypes and KaCalls it makes: -Dkc.cache, true by default. */
        internal val cacheTypes = System.getProperty("kc.cache") != "false"

        private val functionClass = Regex("""kotlin/(coroutines/Suspend)?Function\d+""")

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

        /**
         * The nodes of a tree in preorder, from 1; index 0 is null. This is
         * the numbering of kc's psi node IDs, in which a KDoc comment is a
         * single leaf.
         */
        fun preorder(root: ASTNode): Array<ASTNode> {
            val out = ArrayList<ASTNode?>()
            out.add(null)
            val stack = ArrayDeque<ASTNode>()
            stack.addLast(root)
            while (stack.isNotEmpty()) {
                val n = stack.removeLast()
                out.add(n)
                if (n.elementType == KDocTokens.KDOC) continue
                var c = n.lastChildNode
                while (c != null) {
                    stack.addLast(c)
                    c = c.treePrev
                }
            }
            @Suppress("UNCHECKED_CAST")
            return out.toTypedArray() as Array<ASTNode>
        }
    }
}
