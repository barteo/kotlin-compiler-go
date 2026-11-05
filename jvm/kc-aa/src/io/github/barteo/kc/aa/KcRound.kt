package io.github.barteo.kc.aa

import com.intellij.lang.ASTNode
import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.kdoc.lexer.KDocTokens
import org.jetbrains.kotlin.psi.*
import java.util.IdentityHashMap

/**
 * The symbols of one KSP round: the Go model of the module's sources joined
 * with their PSI, and the class-path classes the round asks for, which
 * [library] loads (the model's `KcOps.cls`). A round is immutable but for
 * the library classes it loads; the next update replaces it and invalidates
 * its [token].
 */
internal class KcRound(files: List<Pair<KcFile, KtFile>>, private val library: (String) -> KcDecl? = { null }) {
    val token = KcToken()

    /** Every source symbol by its pointer key. */
    val byKey = HashMap<String, KaSymbol>()

    /** Source symbols by their PSI, for `KaSession.symbol`. */
    val byPsi = IdentityHashMap<PsiElement, KaSymbol>()

    val classes = HashMap<ClassId, KaNamedClassSymbol>()
    val typeAliases = HashMap<ClassId, KaTypeAliasSymbol>()

    /** The library classes loaded so far, nested ones included. */
    private val libraryClasses = HashMap<ClassId, KaNamedClassSymbol>()

    /** The outermost classes asked of [library], found or not. */
    private val loaded = HashSet<ClassId>()

    /** The classes kc doesn't know, by ID. */
    private val unknownClasses = HashMap<ClassId, KaNamedClassSymbol>()

    /** The class of each annotation entry, for `KtTypeReference.type`. */
    val annotationClasses = IdentityHashMap<KtAnnotationEntry, ClassId>()

    private val annotations = IdentityHashMap<KcAnnotation, KaAnnotation>()

    /**
     * The values of the expressions kc evaluated as annotation arguments
     * or as annotation parameters' defaults, for `evaluateAsAnnotationValue`.
     */
    val values = IdentityHashMap<KtExpression, KaAnnotationValue>()

    /** The types of the type references in declarations, for `KtTypeReference.type`. */
    private val typeRefs = IdentityHashMap<KtTypeReference, Pair<KcType, KcTypeContext>>()

    /** The type kc gave [ref], or null if it isn't among the types of the model's declarations. */
    fun typeOf(ref: KtTypeReference): KaType? = typeRefs[ref]?.let { (t, ctx) -> ctx.type(t) }

    val fileSymbols: List<KcFileSymbol>

    init {
        fileSymbols = files.map { (f, kt) -> Builder(f.path, preorder(kt.node), KaSymbolOrigin.SOURCE).file(f, kt) }
    }

    /** The class [id], or a symbol that knows only its ID if kc doesn't have it. */
    fun classSymbol(id: ClassId): KaNamedClassSymbol =
        classes[id] ?: libraryClass(id) ?: synchronized(this) { unknownClasses.getOrPut(id) { KcLibraryClassSymbol(this, id) } }

    /** The class-path class [id], loading it with its outermost class, or null if there is none. */
    @Synchronized
    fun libraryClass(id: ClassId): KaNamedClassSymbol? {
        libraryClasses[id]?.let { return it }
        if (id.isLocal) return null
        val outer = id.outermostClassId
        if (loaded.add(outer)) {
            library(outer.asString())?.let { d ->
                val origin = if (d.has(KcDecl.JAVA)) KaSymbolOrigin.JAVA_LIBRARY else KaSymbolOrigin.LIBRARY
                Builder(outer.asString(), noNodes, origin).decl(d, null, "lib:" + outer.asString())
            }
        }
        return libraryClasses[id]
    }

    val relations = KcTypeRelations(this)

    private val memberCache = IdentityHashMap<KcClassSymbol, KcClassMembers>()
    private val building = IdentityHashMap<KcClassSymbol, Boolean>()

    /** The member scope of class [c]; empty for a class among its own supertypes. */
    @Synchronized
    fun members(c: KcClassSymbol): KcClassMembers {
        memberCache[c]?.let { return it }
        if (building.put(c, true) != null) return KcClassMembers.EMPTY
        try {
            return KcMemberBuilder(this, c).build().also { memberCache[c] = it }
        } finally {
            building.remove(c)
        }
    }

    /** The declarations [s] directly overrides (see `KaSession.directlyOverriddenSymbols`). */
    fun directlyOverridden(s: KaCallableSymbol): List<KaCallableSymbol> = when (s) {
        is KcFakeOverride -> declaredOriginals(s)
        is KcGetterSymbol -> directlyOverridden(s.property)
        is KcSetterSymbol -> directlyOverridden(s.property)
        is KcFakeGetterSymbol -> directlyOverridden(s.property)
        is KcFakeSetterSymbol -> directlyOverridden(s.property)
        is KcFunctionSymbol -> (s.info.parent as? KcClassSymbol)?.let { members(it).overridden[s] }.orEmpty()
        is KcPropertySymbol -> (s.info.parent as? KcClassSymbol)?.let { members(it).overridden[s] }.orEmpty()
        else -> emptyList()
    }

    /** The declarations [s] overrides at any depth, nearest first. */
    fun allOverridden(s: KaCallableSymbol): List<KaCallableSymbol> {
        val out = ArrayList<KaCallableSymbol>()
        val seen = IdentityHashMap<KaCallableSymbol, Boolean>()
        val queue = ArrayDeque(directlyOverridden(s))
        while (queue.isNotEmpty()) {
            val o = queue.removeFirst()
            if (seen.put(o, true) != null) continue
            out += o
            queue.addAll(directlyOverridden(o))
        }
        return out
    }

    /** The symbol of pointer key [key], loading the library class it is in. */
    fun restore(key: String): KaSymbol? {
        byKey[key]?.let { return it }
        fakeKey.matchEntire(key)?.let { m ->
            val owner = restore(m.groupValues[1]) as? KcClassSymbol ?: return null
            val s = members(owner).callables.getOrNull(m.groupValues[2].toInt()) ?: return null
            val rest = m.groupValues[3]
            return when {
                rest.isEmpty() -> s
                rest == "/get" -> (s as? KaPropertySymbol)?.getter
                rest == "/set" -> (s as? KaPropertySymbol)?.setter
                rest == "/set/value" -> (s as? KaPropertySymbol)?.setter?.parameter
                rest == "/rcv" -> s.receiverParameter
                rest.startsWith("/(") -> (s as? KaFunctionSymbol)?.valueParameters?.getOrNull(rest.drop(2).toInt())
                else -> null
            }
        }
        if (!key.startsWith("lib:")) return null
        // The class's ID comes first; its members' keys add /kind:…, /<i or /(i.
        libraryClass(ClassId.fromString(key.removePrefix("lib:").split(memberKey, 2)[0]))
        return synchronized(this) { byKey[key] }
    }

    fun annotation(a: KcAnnotation): KaAnnotation = annotations.getValue(a)

    /**
     * Builds the symbols of the declarations of a source file, whose psi
     * nodes are [nodes], or of a library class, which has none; [where]
     * names them in errors.
     */
    private inner class Builder(val where: String, val nodes: Array<ASTNode>, val origin: KaSymbolOrigin) {
        /**
         * Where the types of the declarations being built are read; each
         * declaration with type parameters gets its own (see [scoped]).
         */
        var types = KcTypeContext(this@KcRound, ::psi)

        fun file(file: KcFile, kt: KtFile): KcFileSymbol {
            val fs = KcFileSymbol(this@KcRound, file, kt)
            byKey[fs.key] = fs
            byPsi[kt] = fs
            file.annotations.forEach(::annotation)
            fs.declarations = decls(file.decls, fs, fs.key)
            return fs
        }

        /**
         * Runs [build] with a type context for a declaration's type
         * parameters, which [build] declares by setting its typeParams.
         */
        inline fun <T> scoped(build: (KcTypeContext) -> T): T {
            val outer = types
            types = outer.nested()
            try {
                return build(types)
            } finally {
                types = outer
            }
        }

        fun psi(node: Int): PsiElement? {
            if (node == 0) return null
            check(node < nodes.size) { "kc: node $node outside $where (${nodes.size - 1} nodes)" }
            return nodes[node].psi
        }

        fun annotation(a: KcAnnotation) {
            val entry = psi(a.node)?.let {
                it as? KtAnnotationEntry ?: error("kc: annotation node ${a.node} of $where is ${nodes[a.node].elementType}")
            }
            val app = application(a, entry)
            if (entry != null) app.classId?.let { annotationClasses[entry] = it }
            annotations[a] = app
        }

        fun application(a: KcAnnotation, psi: KtCallElement?): KcAnnotationApplication {
            val id = a.classId.takeIf { it.isNotEmpty() }?.let(ClassId::fromString)
            val args = a.args.map { (name, v) -> KcNamedValue(Name.identifier(name), value(v), token) }
            return KcAnnotationApplication(this@KcRound, id, psi, useSiteTarget(a.target), args)
        }

        /** The KaAnnotationValue of [v], recorded as its expression's value. */
        fun value(v: KcValue): KaAnnotationValue {
            val psi = psi(v.node) as? KtElement
            val out = when (v) {
                is KcValue.Const -> KcConstantAnnotationValue(KcConstant.of(v, psi), token)
                is KcValue.Enum -> KcEnumEntryValue(
                    CallableId(ClassId.fromString(v.classId), Name.identifier(v.entry)), psi, token)
                is KcValue.ClassLit -> KcClassLiteralValue(types.type(v.type),
                    v.type.classId.takeIf { v.type.kind == KcType.CLASS && it.isNotEmpty() }?.let(ClassId::fromString),
                    psi, token)
                is KcValue.Nested -> KcNestedAnnotationValue(application(v.annotation, psi as? KtCallElement), psi, token)
                is KcValue.Array -> KcArrayValue(v.elems.map(::value), psi, token)
                is KcValue.Unsupported -> KcUnsupportedValue(psi, token)
            }
            if (psi is KtExpression) values.putIfAbsent(psi, out)
            return out
        }

        fun decls(ds: List<KcDecl>, parent: KaSymbol, parentKey: String): List<KaDeclarationSymbol> =
            ds.mapIndexed { i, d -> decl(d, parent, "$parentKey/${d.kind}:${d.name}:$i") }

        inline fun <reified T : PsiElement> psiOf(d: KcDecl): T? {
            val p = psi(d.node) ?: return null
            return p as? T ?: error("kc: ${d.name} in $where is ${p.javaClass.simpleName}, want ${T::class.simpleName}")
        }

        fun info(d: KcDecl, psi: PsiElement?, parent: KaSymbol?, key: String, annots: List<KcAnnotation> = d.annotations): KcInfo {
            d.annotations.forEach(::annotation)
            if (psi != null) typeRefs(d, psi)
            return KcInfo(this@KcRound, d, psi, parent, key, annots, types, origin)
        }

        /** Records the types of the type references that declaration [d], at [psi], writes. */
        fun typeRefs(d: KcDecl, psi: PsiElement) {
            when (psi) {
                is KtCallableDeclaration -> {
                    typeRef(psi.typeReference, d.type)
                    typeRef(psi.receiverTypeReference, d.receiver)
                }
                is KtClassOrObject -> {
                    // The written supertypes come after the implied kotlin.Enum or kotlin.Annotation.
                    val refs = psi.superTypeListEntries.map { it.typeReference }
                    if (refs.size <= d.supertypes.size) {
                        refs.zip(d.supertypes.takeLast(refs.size)).forEach { (r, t) -> typeRef(r, t) }
                    }
                }
                is KtTypeParameter -> typeRef(psi.extendsBound, d.bounds.firstOrNull())
                is KtTypeAlias -> typeRef(psi.getTypeReference(), d.type)
            }
        }

        fun typeRef(ref: KtTypeReference?, t: KcType?) {
            if (ref == null || t == null) return
            typeRefs[ref] = t to types
            var e = ref.typeElement
            while (e is KtNullableType) e = e.innerType
            val args = t.args.map { it.type }
            when (e) {
                is KtUserType -> {
                    val ps = e.typeArguments
                    if (ps.size == args.size) ps.zip(args).forEach { (p, a) -> typeRef(p.typeReference, a) }
                }
                is KtFunctionType -> {
                    val refs = e.contextReceiversTypeReferences + listOfNotNull(e.receiverTypeReference) +
                        e.parameters.map { it.typeReference } + listOf(e.returnTypeReference)
                    if (refs.size == args.size) refs.zip(args).forEach { (r, a) -> typeRef(r, a) }
                }
            }
        }

        fun <S : KaSymbol> register(s: S, key: String, psi: PsiElement?): S {
            byKey[key] = s
            if (psi != null) byPsi.putIfAbsent(psi, s)
            return s
        }

        /** The symbol of [d]; [parent] is null for a top-level library class. */
        fun decl(d: KcDecl, parent: KaSymbol?, key: String): KaDeclarationSymbol = when (d.kind) {
            KcDecl.CLASS, KcDecl.INTERFACE, KcDecl.OBJECT, KcDecl.ENUM_CLASS, KcDecl.ANNOTATION_CLASS -> scoped { ctx ->
                val psi = psiOf<KtClassOrObject>(d)
                val c = register(KcClassSymbol(info(d, psi, parent, key)), key, psi)
                c.classId?.let { (if (origin == KaSymbolOrigin.SOURCE) classes else libraryClasses)[it] = c }
                c.typeParams = typeParams(d, c, key)
                ctx.typeParams = c.typeParams
                c.members = decls(d.members, c, key)
                c
            }
            KcDecl.ENUM_ENTRY -> {
                val psi = psiOf<KtEnumEntry>(d)
                register(KcEnumEntrySymbol(info(d, psi, parent, key)), key, psi)
            }
            KcDecl.FUNCTION -> scoped { ctx ->
                val psi = psiOf<KtNamedFunction>(d)
                val f = register(KcFunctionSymbol(info(d, psi, parent, key)), key, psi)
                f.typeParams = typeParams(d, f, key)
                ctx.typeParams = f.typeParams
                f.params = params(d, f, key)
                f
            }
            KcDecl.CONSTRUCTOR -> {
                val psi = psiOf<KtConstructor<*>>(d)
                val c = register(KcConstructorSymbol(info(d, psi, parent, key)), key, psi)
                c.params = params(d, c, key)
                c
            }
            KcDecl.PROPERTY -> scoped { ctx -> property(d, parent!!, key, ctx) }
            KcDecl.JAVA_FIELD -> register(KcJavaFieldSymbol(info(d, null, parent, key)), key, null)
            KcDecl.TYPE_ALIAS -> scoped { ctx ->
                val psi = psiOf<KtTypeAlias>(d)
                val t = register(KcTypeAliasSymbol(info(d, psi, parent, key)), key, psi)
                t.classId?.let { typeAliases[it] = t }
                t.typeParams = typeParams(d, t, key)
                ctx.typeParams = t.typeParams
                t
            }
            else -> error("kc: unexpected declaration kind ${d.kind} for ${d.name} in $where")
        }

        /**
         * A property, splitting its annotations by use-site target between
         * the property, its accessors and, for a constructor `val`, the
         * parameter (which gets the untargeted ones, as kotlinc does when
         * the annotation may apply to parameters).
         */
        fun property(d: KcDecl, parent: KaSymbol, key: String, ctx: KcTypeContext): KcPropertySymbol {
            val psi = psi(d.node)
            check(psi == null && origin != KaSymbolOrigin.SOURCE || psi is KtProperty || psi is KtParameter) {
                "kc: property ${d.name} in $where is $psi"
            }
            val fromCtor = psi is KtParameter
            val own = d.annotations.filter {
                when (it.target) {
                    "get", "set", "setparam", "field", "param" -> false
                    "" -> !fromCtor
                    else -> true
                }
            }
            val p = KcPropertySymbol(info(d, psi, parent, key, own))
            register(p, key, psi)
            p.typeParams = typeParams(d, p, key)
            ctx.typeParams = p.typeParams
            val accessors = (psi as? KtProperty)?.accessors.orEmpty()
            p.getterSymbol = KcGetterSymbol(
                KcInfo(this@KcRound, d, accessors.firstOrNull { it.isGetter }, p, "$key/get", d.annotations.filter { it.target == "get" }, types, origin),
                p,
            ).also { register(it, "$key/get", it.info.psi) }
            if (!p.isVal) {
                val setterPsi = accessors.firstOrNull { it.isSetter }
                val s = KcSetterSymbol(
                    KcInfo(this@KcRound, d, setterPsi, p, "$key/set", d.annotations.filter { it.target == "set" }, types, origin),
                    p,
                )
                register(s, "$key/set", setterPsi)
                val paramDecl = KcDecl(KcDecl.VALUE_PARAMETER, setterPsi?.parameter?.name ?: "value", 0, "",
                    visibility = 0, modality = KcDecl.FINAL, flags = 0, variance = KcDecl.INVARIANT,
                    type = d.type, receiver = null, supertypes = emptyList(), bounds = emptyList(),
                    annotations = emptyList(), typeParams = emptyList(), params = emptyList(), members = emptyList())
                s.parameterSymbol = KcValueParameterSymbol(
                    KcInfo(this@KcRound, paramDecl, setterPsi?.parameter, s, "$key/set/value",
                        d.annotations.filter { it.target == "setparam" }, types, origin),
                ).also { register(it, "$key/set/value", setterPsi?.parameter) }
                p.setterSymbol = s
            }
            return p
        }

        fun typeParams(d: KcDecl, parent: KaSymbol, key: String): List<KaTypeParameterSymbol> =
            d.typeParams.mapIndexed { i, t ->
                val k = "$key/<$i"
                val psi = psiOf<KtTypeParameter>(t)
                register(KcTypeParameterSymbol(info(t, psi, parent, k)), k, psi)
            }

        fun params(d: KcDecl, parent: KaSymbol, key: String): List<KaValueParameterSymbol> =
            d.params.mapIndexed { i, v ->
                val k = "$key/($i"
                val psi = psiOf<KtParameter>(v)
                val own = v.annotations.filter { it.target == "" || it.target == "param" }
                val default = v.default?.let(::value)
                register(KcValueParameterSymbol(info(v, psi, parent, k, own), default), k, psi)
            }
    }

    companion object {
        /** The nodes of a library class: none. */
        @Suppress("UNCHECKED_CAST")
        private val noNodes = arrayOfNulls<ASTNode>(1) as Array<ASTNode>

        private val memberKey = Regex("/(?=[0-9<(])")

        /** A fake override's key: its class's key, /~, its index in the member scope, and a member's suffix. */
        private val fakeKey = Regex("^(.*)/~(\\d+)(.*)$")

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
