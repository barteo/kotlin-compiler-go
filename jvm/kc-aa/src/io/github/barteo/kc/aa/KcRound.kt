package io.github.barteo.kc.aa

import com.intellij.lang.ASTNode
import com.intellij.psi.PsiAnnotation
import com.intellij.psi.PsiClass
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiJavaFile
import com.intellij.psi.PsiMethod
import com.intellij.psi.PsiModifier
import com.intellij.psi.PsiModifierListOwner
import com.intellij.psi.PsiTypeParameterListOwner
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaNamedSymbol
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.kdoc.lexer.KDocTokens
import org.jetbrains.kotlin.psi.*
import java.util.IdentityHashMap

/**
 * The symbols of one KSP round: the Go model of the module's sources joined
 * with their PSI, and the class-path classes the round asks for, which
 * [library] loads (the model's `KcOps.cls`), and the class-path packages
 * it asks for, which [packages] loads (`KcOps.pkg`). A round is immutable
 * but for the library declarations it loads; the next update replaces it
 * and invalidates its [token].
 */
internal class KcRound(
    files: List<Pair<KcFile, KtFile>>,
    private val library: (String) -> KcDecl? = { null },
    private val packages: (String) -> KcPackage? = { null },
    /** The module's Java sources, whose classes the model has as library classes. */
    val javaFiles: List<PsiJavaFile> = emptyList(),
) {
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

    /** The classes of [javaFiles] by their model class IDs. */
    private val javaClasses = HashMap<String, PsiClass>().also { m ->
        fun walk(c: PsiClass, id: String) {
            m[id] = c
            for (n in c.innerClasses) n.name?.let { walk(n, "$id.$it") }
        }
        for (f in javaFiles) {
            val pkg = f.packageName.replace('.', '/')
            for (c in f.classes) c.name?.let { walk(c, if (pkg.isEmpty()) it else "$pkg/$it") }
        }
    }

    /** The class ID of [c], a class of [javaFiles], or null if it isn't one. */
    fun javaClassId(c: PsiClass): ClassId? {
        val name = c.name ?: return null
        val outer = c.containingClass
        if (outer != null) return javaClassId(outer)?.createNestedClassId(Name.identifier(name))
        val file = c.containingFile as? PsiJavaFile ?: return null
        return ClassId(FqName(file.packageName), Name.identifier(name))
    }

    /**
     * The class of Java annotation [a], found as javac finds a type name:
     * among the classes nested in the classes around it, then the file's
     * single-type imports, its package, its on-demand imports, `java.lang`,
     * and last as a qualified name. Java PSI resolves only Java classes, so
     * KSP asks for the Kotlin ones here.
     */
    fun javaAnnotationClass(a: PsiAnnotation): ClassId? {
        val name = a.nameReferenceElement?.text?.filterNot { it.isWhitespace() } ?: return null
        val file = a.containingFile as? PsiJavaFile ?: return null
        val first = name.substringBefore('.')
        val rest = name.substring(first.length)
        val candidates = ArrayList<String>()
        var c = generateSequence(a.parent) { it.parent }.firstOrNull { it is PsiClass } as PsiClass?
        while (c != null) {
            javaClassId(c)?.let { candidates += it.asFqNameString() + "." + name }
            c = c.containingClass
        }
        val imports = file.importList?.importStatements.orEmpty()
        imports.filter { !it.isOnDemand }.mapNotNull { it.qualifiedName }
            .filter { it == first || it.endsWith(".$first") }.forEach { candidates += it + rest }
        candidates += if (file.packageName.isEmpty()) name else file.packageName + "." + name
        imports.filter { it.isOnDemand }.mapNotNull { it.qualifiedName }.forEach { candidates += "$it.$name" }
        candidates += "java.lang.$name"
        candidates += name
        for (fq in candidates) classIdOf(fq)?.let { return it }
        return null
    }

    /** The class qualified name [fq] names, trying each split into package and classes. */
    private fun classIdOf(fq: String): ClassId? {
        val segs = fq.split('.')
        for (i in segs.indices.reversed()) {
            val id = ClassId(FqName.fromSegments(segs.subList(0, i)), FqName.fromSegments(segs.subList(i, segs.size)), false)
            if (classes[id] != null || libraryClass(id) != null) return id
        }
        return null
    }

    /** The sources' packages and the packages they are in. */
    private val sourcePackages = HashSet<FqName>()

    init {
        // Walking a file's tree has IntelliJ parse it, the first time:
        // most of a session's first round. Files parse independently.
        val trees = files.parallelStream().map { (_, kt) -> preorder(kt.node) }.toList()
        fileSymbols = files.mapIndexed { i, (f, kt) -> Builder(f.path, trees[i], KaSymbolOrigin.SOURCE).file(f, kt) }
        for ((f, _) in files) {
            var fq = FqName(f.packageName)
            while (sourcePackages.add(fq) && !fq.isRoot) fq = fq.parent()
        }
    }

    /** A class-path package: its classes' IDs and its top-level functions, properties and type aliases. */
    class LibraryPackage(val classes: List<ClassId>, val decls: List<KaDeclarationSymbol>)

    private val libraryPackages = HashMap<FqName, LibraryPackage?>()
    private val packageSymbols = HashMap<FqName, KcPackageSymbol>()
    private val packageScopes = HashMap<FqName, List<KaDeclarationSymbol>>()

    @Synchronized
    fun packageSymbol(fq: FqName): KcPackageSymbol = packageSymbols.getOrPut(fq) { KcPackageSymbol(this, fq) }

    /** Whether package [fq], or a package in it, has a source file or a class-path class. */
    fun packageExists(fq: FqName) = fq in sourcePackages || libraryPackage(fq) != null

    /** The class-path package [fq], loading its top-level declarations, or null if there is none. */
    @Synchronized
    fun libraryPackage(fq: FqName): LibraryPackage? {
        if (libraryPackages.containsKey(fq)) return libraryPackages[fq]
        val p = packages(fq.asString())?.let { kp ->
            val key = "pkg:" + fq.asString()
            val decls = Builder(key, noNodes, KaSymbolOrigin.LIBRARY).decls(kp.decls, packageSymbol(fq), key)
            LibraryPackage(kp.classes.map(ClassId::fromString), decls)
        }
        libraryPackages[fq] = p
        return p
    }

    /**
     * The declarations of package [fq]: the sources' top-level ones, then
     * the class path's top-level functions, properties and type aliases,
     * then its classes.
     */
    fun packageDeclarations(fq: FqName): List<KaDeclarationSymbol> {
        synchronized(this) { packageScopes[fq]?.let { return it } }
        val name = fq.asString()
        val out = ArrayList<KaDeclarationSymbol>()
        for (f in fileSymbols) if (f.file.packageName == name) out += f.declarations
        libraryPackage(fq)?.let { p ->
            out += p.decls
            p.classes.mapNotNullTo(out) { if (it in classes) null else libraryClass(it) }
        }
        return synchronized(this) { packageScopes.getOrPut(fq) { out } }
    }

    /** The top-level functions and properties named [name] in package [fq], the sources' first. */
    fun topLevelCallables(fq: FqName, name: Name): List<KaCallableSymbol> {
        val pkg = fq.asString()
        val out = ArrayList<KaCallableSymbol>()
        for (f in fileSymbols) if (f.file.packageName == pkg) {
            f.declarations.filterIsInstanceTo<KaCallableSymbol, _>(out)
        }
        libraryPackage(fq)?.decls?.filterIsInstanceTo<KaCallableSymbol, _>(out)
        return out.filter { (it as? KaNamedSymbol)?.name == name }
    }

    /** The type alias [id], of the sources or of the class path, or null if there is none. */
    fun typeAlias(id: ClassId): KaTypeAliasSymbol? {
        typeAliases[id]?.let { return it }
        if (id.isLocal || id.isNestedClass) return null
        libraryPackage(id.packageFqName)
        return synchronized(this) { typeAliases[id] }
    }

    /**
     * [t] with its typealias types expanded: a type with [KcClassType.isAlias]
     * becomes its typealias's expansion of its arguments, abbreviated by it.
     */
    fun expand(t: KaType): KaType {
        var cur = t
        var depth = 0
        while (cur is KcClassType && cur.isAlias && depth++ < 16) {
            val alias = typeAlias(cur.classId) ?: return cur
            val params = alias.typeParameters
            val sub = if (params.isEmpty() || params.size != cur.typeArguments.size) relations.empty
            else relations.substitutor(params.zip(cur.typeArguments).associate { (p, a) ->
                p to (a.type ?: p.upperBounds.firstOrNull() ?: relations.anyType(true))
            })
            val e = sub.substitute(alias.expandedType)
            val nullable = cur.marked || (e as? KcTypeBase)?.marked == true
            cur = when (e) {
                is KcClassType -> KcClassType(this, e.classId, e.typeArguments, nullable, cur.withNullability(nullable), e.isAlias)
                is KcFunctionType ->
                    KcFunctionType(this, e.classId, e.typeArguments, nullable, e.hasReceiver, e.contexts, cur.withNullability(nullable))
                else -> return (e as? KcTypeBase)?.withNullability(nullable) ?: e
            }
        }
        return cur
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
                val origin = when {
                    d.has(KcDecl.JAVA_SOURCE) -> KaSymbolOrigin.JAVA_SOURCE
                    d.has(KcDecl.JAVA) -> KaSymbolOrigin.JAVA_LIBRARY
                    else -> KaSymbolOrigin.LIBRARY
                }
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
        if (key.startsWith("pkgsym:")) return packageSymbol(FqName(key.removePrefix("pkgsym:")))
        if (key.startsWith("pkg:")) {
            libraryPackage(FqName(key.removePrefix("pkg:").split(memberKey, 2)[0]))
            return synchronized(this) { byKey[key] }
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
            return KcAnnotationApplication(this@KcRound, id, psi, useSiteTarget(if (a.implied) "" else a.target), args)
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

        /** The members of each Java class matched to symbols so far. */
        val javaUsed = IdentityHashMap<PsiElement, Boolean>()

        /**
         * The Java PSI of [d], a declaration of the Java sources in [parent]:
         * a class by its ID, a method or constructor by name, parameter
         * count and visibility, a field or enum constant by name. A
         * constructor Java doesn't write is its class, as in FIR.
         */
        fun javaPsi(d: KcDecl, parent: KaSymbol?): PsiElement? {
            if (origin != KaSymbolOrigin.JAVA_SOURCE) return null
            val owner = (parent as? KcClassSymbol)?.info?.psi as? PsiClass
            return when (d.kind) {
                KcDecl.CLASS, KcDecl.INTERFACE, KcDecl.OBJECT, KcDecl.ENUM_CLASS, KcDecl.ANNOTATION_CLASS ->
                    javaClasses[d.classId]
                KcDecl.FUNCTION, KcDecl.CONSTRUCTOR -> {
                    owner ?: return null
                    val ctor = d.kind == KcDecl.CONSTRUCTOR
                    val ms = if (ctor) owner.constructors else owner.findMethodsByName(d.name, false)
                    ms.firstOrNull { it !in javaUsed && it.parameterList.parametersCount == d.params.size && javaVisibility(it) == d.visibility }
                        ?.also { javaUsed[it] = true }
                        ?: if (ctor) owner else null
                }
                KcDecl.JAVA_FIELD, KcDecl.ENUM_ENTRY -> owner?.findFieldByName(d.name, false)
                else -> null
            }
        }

        /** The Java PSI of [parent], a Java source declaration, or null. */
        fun javaOwnerPsi(parent: KaSymbol): PsiElement? = if (origin != KaSymbolOrigin.JAVA_SOURCE) null else when (parent) {
            is KcClassSymbol -> parent.info.psi
            is KcFunctionSymbol -> parent.info.psi
            is KcConstructorSymbol -> parent.info.psi
            else -> null
        }

        fun javaVisibility(m: PsiModifierListOwner): Int = when {
            m.hasModifierProperty(PsiModifier.PUBLIC) -> KcDecl.PUBLIC
            m.hasModifierProperty(PsiModifier.PROTECTED) -> KcDecl.PROTECTED
            m.hasModifierProperty(PsiModifier.PRIVATE) -> KcDecl.PRIVATE
            m is PsiMethod && m.containingClass?.isInterface == true -> KcDecl.PUBLIC
            else -> KcDecl.PACKAGE_PRIVATE
        }

        inline fun <reified T : PsiElement> psiOf(d: KcDecl): T? {
            val p = psi(d.node) ?: return null
            return p as? T ?: error("kc: ${d.name} in $where is ${p.javaClass.simpleName}, want ${T::class.simpleName}")
        }

        fun info(d: KcDecl, psi: PsiElement?, parent: KaSymbol?, key: String, annots: List<KcAnnotation> = d.annotations): KcInfo {
            d.annotations.forEach(::annotation)
            if (psi != null) typeRefs(d, psi)
            // A source declaration without a node is one Kotlin generates:
            // an implicit constructor, an enum class's values().
            val o = if (origin == KaSymbolOrigin.SOURCE && d.node == 0) KaSymbolOrigin.SOURCE_MEMBER_GENERATED else origin
            return KcInfo(this@KcRound, d, psi, parent, key, annots, types, o)
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
                val psi = psiOf<KtClassOrObject>(d) ?: javaPsi(d, parent)
                val c = register(KcClassSymbol(info(d, psi, parent, key)), key, psi)
                c.classId?.let { (if (origin == KaSymbolOrigin.SOURCE) classes else libraryClasses)[it] = c }
                c.typeParams = typeParams(d, c, key)
                ctx.typeParams = c.typeParams
                c.members = decls(d.members, c, key)
                c
            }
            KcDecl.ENUM_ENTRY -> {
                val psi = psiOf<KtEnumEntry>(d) ?: javaPsi(d, parent)
                register(KcEnumEntrySymbol(info(d, psi, parent, key)), key, psi)
            }
            KcDecl.FUNCTION -> scoped { ctx ->
                val psi = psiOf<KtNamedFunction>(d) ?: javaPsi(d, parent)
                val f = register(KcFunctionSymbol(info(d, psi, parent, key)), key, psi)
                f.typeParams = typeParams(d, f, key)
                ctx.typeParams = f.typeParams
                f.params = params(d, f, key)
                f
            }
            KcDecl.CONSTRUCTOR -> {
                val psi = psiOf<KtConstructor<*>>(d) ?: javaPsi(d, parent)
                val c = register(KcConstructorSymbol(info(d, psi, parent, key)), key, psi)
                c.params = params(d, c, key)
                c
            }
            KcDecl.PROPERTY -> scoped { ctx -> property(d, parent!!, key, ctx) }
            KcDecl.JAVA_FIELD -> {
                val psi = javaPsi(d, parent)
                register(KcJavaFieldSymbol(info(d, psi, parent, key)), key, psi)
            }
            KcDecl.TYPE_ALIAS -> scoped { ctx ->
                val psi = psiOf<KtTypeAlias>(d)
                val t = register(KcTypeAliasSymbol(info(d, psi, parent, key)), key, psi)
                t.classId?.let { typeAliases.putIfAbsent(it, t) }
                t.typeParams = typeParams(d, t, key)
                ctx.typeParams = t.typeParams
                t
            }
            else -> error("kc: unexpected declaration kind ${d.kind} for ${d.name} in $where")
        }

        /**
         * A property, splitting its annotations by use-site target between
         * the property and its accessors. For a constructor `val`, the model
         * already has only the property's: the untargeted ones that may be
         * on a property are on both it and the parameter, as in Kotlin 2.4.
         */
        fun property(d: KcDecl, parent: KaSymbol, key: String, ctx: KcTypeContext): KcPropertySymbol {
            val psi = psi(d.node)
            check(psi == null && (origin != KaSymbolOrigin.SOURCE || d.node == 0) || psi is KtProperty || psi is KtParameter) {
                "kc: property ${d.name} in $where is $psi"
            }
            val own = d.annotations.filter {
                when (it.target) {
                    "get", "set", "setparam", "field", "param" -> false
                    else -> true
                }
            }
            val p = KcPropertySymbol(info(d, psi, parent, key, own))
            register(p, key, psi)
            p.typeParams = typeParams(d, p, key)
            ctx.typeParams = p.typeParams
            val accessors = (psi as? KtProperty)?.accessors.orEmpty()
            p.getterSymbol = KcGetterSymbol(
                KcInfo(this@KcRound, d, accessors.firstOrNull { it.isGetter }, p, "$key/get", d.annotations.filter { it.target == "get" }, types, p.info.origin),
                p,
            ).also { register(it, "$key/get", it.info.psi) }
            if (!p.isVal) {
                val setterPsi = accessors.firstOrNull { it.isSetter }
                val s = KcSetterSymbol(
                    KcInfo(this@KcRound, d, setterPsi, p, "$key/set", d.annotations.filter { it.target == "set" }, types, p.info.origin),
                    p,
                )
                register(s, "$key/set", setterPsi)
                val paramDecl = KcDecl(KcDecl.VALUE_PARAMETER, setterPsi?.parameter?.name ?: "value", 0, "",
                    visibility = 0, modality = KcDecl.FINAL, flags = 0, variance = KcDecl.INVARIANT,
                    type = d.type, receiver = null, supertypes = emptyList(), bounds = emptyList(),
                    annotations = emptyList(), typeParams = emptyList(), params = emptyList(), members = emptyList())
                s.parameterSymbol = KcValueParameterSymbol(
                    KcInfo(this@KcRound, paramDecl, setterPsi?.parameter, s, "$key/set/value",
                        d.annotations.filter { it.target == "setparam" }, types, p.info.origin),
                ).also { register(it, "$key/set/value", setterPsi?.parameter) }
                p.setterSymbol = s
            }
            if (p.hasBackingField) {
                val field = d.annotations.filter { it.target == "field" || it.target == "delegate" }
                p.backingField = KcBackingFieldSymbol(
                    KcInfo(this@KcRound, d, null, p, "$key/field", field, types, p.info.origin), p,
                ).also { register(it, "$key/field", null) }
            }
            return p
        }

        fun typeParams(d: KcDecl, parent: KaSymbol, key: String): List<KaTypeParameterSymbol> =
            d.typeParams.mapIndexed { i, t ->
                val k = "$key/<$i"
                val psi = psiOf<KtTypeParameter>(t)
                    ?: (javaOwnerPsi(parent) as? PsiTypeParameterListOwner)?.typeParameters?.getOrNull(i)
                register(KcTypeParameterSymbol(info(t, psi, parent, k)), k, psi)
            }

        fun params(d: KcDecl, parent: KaSymbol, key: String): List<KaValueParameterSymbol> =
            d.params.mapIndexed { i, v ->
                val k = "$key/($i"
                val psi = psiOf<KtParameter>(v)
                    ?: (javaOwnerPsi(parent) as? PsiMethod)?.parameterList?.parameters?.getOrNull(i)
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
