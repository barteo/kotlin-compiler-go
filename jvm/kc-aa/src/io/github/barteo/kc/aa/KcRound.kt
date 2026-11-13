package io.github.barteo.kc.aa

import com.intellij.lang.ASTNode
import com.intellij.psi.PsiAnnotation
import com.intellij.psi.PsiClass
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiJavaFile
import com.intellij.psi.PsiMethod
import com.intellij.psi.PsiModifier
import com.intellij.psi.PsiModifierListOwner
import com.intellij.psi.PsiReferenceExpression
import com.intellij.psi.PsiTypeParameterListOwner
import com.intellij.psi.util.PsiTreeUtil
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaNamedSymbol
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.analysis.api.KaConstantInitializerValue
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.fileClasses.JvmFileClassUtil
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
    /** Where Java library classes' class-file PSI is found; none without it. */
    private val project: com.intellij.openapi.project.Project? = null,
    /** The errors kc found in the module's sources. */
    val diagnostics: List<KcDiagnostic> = emptyList(),
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
    /** The typealiases annotation entries name their classes by. */
    val annotationAliases = IdentityHashMap<KtAnnotationEntry, ClassId>()

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
    fun javaAnnotationClass(a: PsiAnnotation): ClassId? =
        javaClassNamed(a.nameReferenceElement?.text ?: return null, a)

    /**
     * The Kotlin enum entry a Java reference `E.X` names, which Java PSI
     * doesn't resolve: an annotation argument's, as KSP reads it.
     */
    fun javaEnumEntry(ref: PsiReferenceExpression): CallableId? {
        val q = ref.qualifierExpression ?: return null
        val cls = javaClassNamed(q.text, ref) ?: return null
        return CallableId(cls, Name.identifier(ref.referenceName ?: return null))
    }

    /**
     * The value of the Kotlin `const val` a Java reference `C.X` names,
     * which Java PSI doesn't resolve: one in a source file's facade class
     * (`ConstKt.X`), or in a source class or its companion. An annotation
     * argument's, as KSP reads it.
     */
    @OptIn(KaExperimentalApi::class)
    fun javaConstant(ref: PsiReferenceExpression): Any? {
        val q = ref.qualifierExpression ?: return null
        val name = ref.referenceName ?: return null
        for (fq in javaNameCandidates(q.text, ref)) {
            val members = facades[FqName(fq)]?.declarations
                ?: classIdOf(fq)?.let { id ->
                    val c = classSymbol(id) as? KcClassSymbol ?: return null
                    declared(c) + (c.companionObject as? KcClassSymbol)?.let(::declared).orEmpty()
                }
                ?: continue
            val p = members.firstOrNull { it is KcPropertySymbol && it.isConst && it.name.asString() == name }
                as? KcPropertySymbol ?: return null
            return (p.initializer as? KaConstantInitializerValue)?.constant?.value
        }
        return null
    }

    /** The source files by their facade classes' names. */
    private val facades: Map<FqName, KcFileSymbol> by lazy {
        fileSymbols.associateBy { JvmFileClassUtil.getFileClassInfoNoResolve(it.psi).facadeClassFqName }
    }

    /** The class Java type name [text] in [context] names; see [javaAnnotationClass]. */
    fun javaClassNamed(text: String, context: PsiElement): ClassId? {
        for (fq in javaNameCandidates(text, context)) classIdOf(fq)?.let { return it }
        return null
    }

    /** The qualified names Java type name [text] in [context] may stand for, in javac's order. */
    private fun javaNameCandidates(text: String, context: PsiElement): List<String> {
        val name = text.filterNot { it.isWhitespace() }
        val file = context.containingFile as? PsiJavaFile ?: return emptyList()
        val first = name.substringBefore('.')
        val rest = name.substring(first.length)
        val candidates = ArrayList<String>()
        var c = generateSequence(context.parent) { it.parent }.firstOrNull { it is PsiClass } as PsiClass?
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
        return candidates
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

    /**
     * [t], a supertype of Java source class [c], with the type annotations
     * its extends or implements clause writes, `implements @A Parent`.
     * The Go model keeps a Java source's annotation names only, so they
     * are read from the PSI, with node 0.
     */
    fun javaSupertype(c: PsiClass, t: KcType): KcType {
        if (t.kind != KcType.CLASS || t.annotations.isNotEmpty()) return t
        val simple = t.classId.substringAfterLast('/').substringAfterLast('.')
        val refs = listOfNotNull(c.extendsList, c.implementsList).flatMap { it.referenceElements.asList() }
        val ref = refs.firstOrNull { it.referenceName == simple } ?: return t
        val annots = PsiTreeUtil.getChildrenOfTypeAsList(ref, PsiAnnotation::class.java)
        if (annots.isEmpty()) return t
        return KcType(t.kind, t.classId, t.flags, t.param, t.name, t.contexts, t.args, t.abbrev, annots.map(::javaAnnotation))
    }

    /** The model's form of Java annotation [a], with node 0. */
    fun javaAnnotation(a: PsiAnnotation): KcAnnotation = KcAnnotation(
        javaAnnotationClass(a)?.asString() ?: "", 0, "", false,
        a.parameterList.attributes.mapNotNull { at -> at.value?.let { (at.name ?: "value") to javaValue(it) } })

    private fun javaValue(v: com.intellij.psi.PsiAnnotationMemberValue): KcValue = when (v) {
        is PsiAnnotation -> KcValue.Nested(0, javaAnnotation(v))
        is com.intellij.psi.PsiArrayInitializerMemberValue -> KcValue.Array(0, v.initializers.map(::javaValue))
        is PsiReferenceExpression -> when (val r = v.resolve()) {
            is com.intellij.psi.PsiEnumConstant -> r.containingClass?.let(::javaClassId)
                ?.let { KcValue.Enum(0, it.asString(), r.name) } ?: KcValue.Unsupported(0)
            null -> javaEnumEntry(v)?.let { KcValue.Enum(0, it.classId!!.asString(), it.callableName.asString()) }
                ?: javaConstant(v)?.let(::constValue)
                ?: KcValue.Unsupported(0)
            else -> evaluate(v)
        }
        is com.intellij.psi.PsiExpression -> evaluate(v)
        else -> KcValue.Unsupported(0)
    }

    private fun evaluate(e: com.intellij.psi.PsiExpression): KcValue =
        constValue(com.intellij.psi.JavaPsiFacade.getInstance(e.project).constantEvaluationHelper.computeConstantExpression(e))

    private fun constValue(c: Any?): KcValue {
        fun k(kind: Int, bits: Long = 0, float: Double = 0.0, str: String = "") = KcValue.Const(0, kind, bits, float, str)
        return when (c) {
            is Boolean -> k(KcValue.Const.BOOLEAN, if (c) 1 else 0)
            is Char -> k(KcValue.Const.CHAR, c.code.toLong())
            is Byte -> k(KcValue.Const.BYTE, c.toLong())
            is Short -> k(KcValue.Const.SHORT, c.toLong())
            is Int -> k(KcValue.Const.INT, c.toLong())
            is Long -> k(KcValue.Const.LONG, c)
            is Float -> k(KcValue.Const.FLOAT, float = c.toDouble())
            is Double -> k(KcValue.Const.DOUBLE, float = c)
            is String -> k(KcValue.Const.STRING, str = c)
            else -> KcValue.Unsupported(0)
        }
    }

    /** The sources' packages and the packages they are in. */
    private val sourcePackages = HashSet<FqName>()

    init {
        // Walking a file's tree has IntelliJ parse it, the first time:
        // most of a session's first round. Files parse independently.
        // kc's tree and IntelliJ's must have the same shape for the preorder
        // index to join them: the same kinds in the same order.
        val trees = files.parallelStream().map { (f, kt) ->
            preorder(kt.node).also { nodes ->
                if (checkKinds && f.nodes >= 0 && (nodes.size - 1 != f.nodes || KcKinds.hash(nodes) != f.kinds))
                    throw KcPsiMismatch(f.path, f.nodes, nodes.size - 1)
            }
        }.toList()
        fileSymbols = files.mapIndexed { i, (f, kt) -> Builder(f.path, trees[i], KaSymbolOrigin.SOURCE).file(f, kt) }
        for (name in files.map { it.first.packageName } + javaFiles.map { it.packageName }) {
            var fq = FqName(name)
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
     * The declarations of package [fq], callables first, as in FIR: the
     * sources' top-level functions and properties, the class path's, then
     * the Kotlin sources' classes and type aliases, the Java sources'
     * classes (in hash order, as in FIR), and the class path's classes and
     * type aliases.
     */
    fun packageDeclarations(fq: FqName): List<KaDeclarationSymbol> {
        synchronized(this) { packageScopes[fq]?.let { return it } }
        val name = fq.asString()
        val sources = fileSymbols.filter { it.file.packageName == name }.flatMap { it.declarations }
        val lib = libraryPackage(fq)
        val out = ArrayList<KaDeclarationSymbol>()
        sources.filterTo(out) { it is KaCallableSymbol }
        lib?.decls?.filterTo(out) { it is KaCallableSymbol }
        sources.filterTo(out) { it !is KaCallableSymbol }
        // FIR lists them in the order of the fastutil set the CLI Java file
        // manager collects their names in (knownClassNamesInPackage).
        val javaNames = it.unimi.dsi.fastutil.objects.ObjectOpenHashSet<String>()
        for (f in javaFiles) if (f.packageName == name) f.classes.mapNotNullTo(javaNames) { it.name }
        for (n in javaNames) libraryClass(ClassId(fq, Name.identifier(n)))?.let { out += it }
        lib?.let { p -> p.classes.mapNotNullTo(out) { if (it in classes) null else libraryClass(it) } }
        lib?.decls?.filterTo(out) { it !is KaCallableSymbol }
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

    private val classFiles = HashMap<String, PsiClass?>()

    /**
     * The class-file PSI of Java library class [id] (a model class ID), as
     * FIR has it: KSP gives such a class its .class file as containing file.
     */
    @Synchronized
    fun classFilePsi(id: String): PsiClass? {
        if (id in classFiles) return classFiles[id]
        val p = project ?: return null
        val fq = ClassId.fromString(id).asSingleFqName().asString()
        val c = com.intellij.psi.JavaPsiFacade.getInstance(p).findClass(fq, com.intellij.psi.search.GlobalSearchScope.allScope(p))
            ?.takeIf { it is com.intellij.psi.PsiCompiledElement }
        classFiles[id] = c
        return c
    }

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

    /**
     * Java method [s]'s [KcEnhancement] in its class [owner], or null:
     * none while that class's members are being built.
     */
    fun enhancement(s: KaCallableSymbol, owner: KaSymbol?): KcEnhancement? {
        val c = owner as? KcClassSymbol ?: return null
        if (building.containsKey(c)) return null
        return members(c).enhanced[s]
    }

    /** The declarations of class [c]: as FIR has them for a Java class (see `KcMemberBuilder.javaDeclared`). */
    fun declared(c: KcClassSymbol): List<KaDeclarationSymbol> = members(c).declared ?: c.members

    /** The declarations [s] directly overrides (see `KaSession.directlyOverriddenSymbols`). */
    fun directlyOverridden(s: KaCallableSymbol): List<KaCallableSymbol> = when (s) {
        is KcFakeOverride -> declaredOriginals(s)
        is KcGetterSymbol -> directlyOverridden(s.property)
        is KcSetterSymbol -> directlyOverridden(s.property)
        is KcFakeGetterSymbol -> directlyOverridden(s.property)
        is KcFakeSetterSymbol -> directlyOverridden(s.property)
        is KcSyntheticGetterSymbol -> directlyOverridden(s.property)
        is KcSyntheticSetterSymbol -> directlyOverridden(s.property)
        is KcSyntheticPropertySymbol -> members(s.owner).overridden[s].orEmpty()
        is KcRenamedFunctionSymbol -> members(s.owner).overridden[s].orEmpty()
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
        (declaredKey.matchEntire(key) ?: fakeKey.matchEntire(key))?.let { m ->
            val owner = restore(m.groupValues[1]) as? KcClassSymbol ?: return null
            val list = if (declaredKey.matches(key)) declared(owner) else members(owner).callables
            val s = list.getOrNull(m.groupValues[2].toInt()) as? KaCallableSymbol ?: return null
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
        var types = KcTypeContext(this@KcRound, ::psi, annotate = ::typeAnnotation)

        /**
         * The KaAnnotation of a type's annotation [a], built the first time
         * the type is, which may be after this builder's declarations.
         */
        fun typeAnnotation(a: KcAnnotation): KaAnnotation = synchronized(this@KcRound) {
            annotations[a] ?: run { annotation(a); annotations.getValue(a) }
        }

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

        /**
         * Whether [a] is written on the receiver type of declaration [decl],
         * which makes it the receiver parameter's: a library one, or a
         * source one inside the receiver type reference. A `@receiver:`
         * annotation on the declaration stays the declaration's, as in FIR.
         */
        fun onReceiverType(a: KcAnnotation, decl: PsiElement?): Boolean {
            if (a.target != "receiver") return false
            val e = psi(a.node) ?: return true
            return PsiTreeUtil.isAncestor((decl as? KtCallableDeclaration)?.receiverTypeReference, e, false)
        }

        fun annotation(a: KcAnnotation) {
            val entry = psi(a.node)?.let {
                it as? KtAnnotationEntry ?: error("kc: annotation node ${a.node} of $where is ${nodes[a.node].elementType}")
            }
            val app = application(a, entry)
            if (entry != null) {
                app.classId?.let { annotationClasses[entry] = it }
                if (a.alias.isNotEmpty()) annotationAliases[entry] = ClassId.fromString(a.alias)
            }
            val projections = (entry?.typeReference?.typeElement as? KtUserType)?.typeArguments.orEmpty()
            if (projections.size == a.typeArgs.size) projections.zip(a.typeArgs).forEach { (p, t) -> typeRef(p.typeReference, t) }
            annotations[a] = app
        }

        fun application(a: KcAnnotation, psi: KtCallElement?): KcAnnotationApplication {
            val id = a.classId.takeIf { it.isNotEmpty() }?.let(ClassId::fromString)
            val args = a.args.map { (name, v) -> KcNamedValue(Name.identifier(name), value(v), token) }
            return KcAnnotationApplication(this@KcRound, id, psi, useSiteTarget(if (a.implied) "" else a.target), args)
        }

        /** The type of a class literal of type [t]: a generic class's
         *  without arguments, as `HashMap::class`, has star arguments. */
        fun classLiteralType(t: KaType): KaType {
            if (t !is KcClassType || t.typeArguments.isNotEmpty() || t.isAlias) return t
            val n = (t.symbol as? KaClassSymbol)?.typeParameters?.size ?: 0
            return if (n == 0) t else KcClassType(this@KcRound, t.classId, List(n) { KcStarProjection(token) }, t.marked)
        }

        /** The KaAnnotationValue of [v], recorded as its expression's value. */
        fun value(v: KcValue): KaAnnotationValue {
            val psi = psi(v.node) as? KtElement
            val out = when (v) {
                is KcValue.Const -> KcConstantAnnotationValue(KcConstant.of(v, psi), token)
                is KcValue.Enum -> KcEnumEntryValue(
                    CallableId(ClassId.fromString(v.classId), Name.identifier(v.entry)), psi, token)
                is KcValue.ClassLit -> KcClassLiteralValue(classLiteralType(types.type(v.type)),
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

        fun info(d: KcDecl, psi: PsiElement?, parent: KaSymbol?, key: String,
                 annots: List<KcAnnotation> = d.annotations.filter { !onReceiverType(it, psi) },
                 findPsi: (() -> PsiElement?)? = null): KcInfo {
            d.annotations.forEach(::annotation)
            if (psi != null) typeRefs(d, psi)
            // A source declaration without a node is one Kotlin generates:
            // an implicit constructor, an enum class's values().
            val o = if (origin == KaSymbolOrigin.SOURCE && d.node == 0 || javaEnumGenerated(d, parent)) KaSymbolOrigin.SOURCE_MEMBER_GENERATED else origin
            return KcInfo(this@KcRound, d, psi, parent, key, annots, types, o, findPsi)
        }

        /**
         * Whether [d] is a member FIR generates for a Java enum: values()
         * and valueOf() with its parameter, and a source enum's entries (a library one's keeps
         * the class's origin, as in FIR). A Java enum can't declare these.
         */
        fun javaEnumGenerated(d: KcDecl, parent: KaSymbol?): Boolean {
            if (origin != KaSymbolOrigin.JAVA_SOURCE && origin != KaSymbolOrigin.JAVA_LIBRARY) return false
            if (d.kind == KcDecl.VALUE_PARAMETER) return parent is KcFunctionSymbol && parent.origin == KaSymbolOrigin.SOURCE_MEMBER_GENERATED
            if ((parent as? KcClassSymbol)?.classKind != KaClassKind.ENUM_CLASS || !d.has(KcDecl.STATIC)) return false
            return when (d.kind) {
                KcDecl.FUNCTION -> d.name == "values" && d.params.isEmpty() || d.name == "valueOf" && d.params.size == 1
                KcDecl.PROPERTY -> d.name == "entries" && origin == KaSymbolOrigin.JAVA_SOURCE
                else -> false
            }
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
            // Registered now, so the entries' classes are known before the types are built.
            t.annotations.forEach(::typeAnnotation)
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
                val id = d.classId
                val classFile = if (origin == KaSymbolOrigin.JAVA_LIBRARY && id.isNotEmpty()) { { classFilePsi(id) } } else null
                val c = register(KcClassSymbol(info(d, psi, parent, key, findPsi = classFile)), key, psi)
                c.classId?.let { (if (origin == KaSymbolOrigin.SOURCE) classes else libraryClasses)[it] = c }
                c.typeParams = typeParams(d, c, key)
                ctx.typeParams = c.typeParams
                c.members = decls(d.members, c, key)
                c.generated = generated(d, c, key)
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
                f.contexts = contexts(d, f, key)
                locals(d, key) { f }
                f
            }
            KcDecl.CONSTRUCTOR -> {
                val psi = psiOf<KtConstructor<*>>(d) ?: javaPsi(d, parent)
                val c = register(KcConstructorSymbol(info(d, psi, parent, key)), key, psi)
                c.params = params(d, c, key)
                locals(d, key) { c }
                c
            }
            KcDecl.PROPERTY -> scoped { ctx -> property(d, parent!!, key, ctx) }
            KcDecl.JAVA_FIELD -> {
                val psi = javaPsi(d, parent)
                // A Java library field's is its class file's, as in FIR:
                // KSP reads a backing field's modifiers from it.
                val classFile = if (origin == KaSymbolOrigin.JAVA_LIBRARY) {
                    { (parent?.psi as? PsiClass)?.findFieldByName(d.name, false) }
                } else null
                register(KcJavaFieldSymbol(info(d, psi, parent, key, findPsi = classFile)), key, psi)
            }
            KcDecl.DESTRUCTURING -> {
                val psi = psiOf<KtDestructuringDeclaration>(d)
                val s = register(KcDestructuringSymbol(info(d, psi, parent, key)), key, psi)
                // The entries' containing symbol is the enclosing callable,
                // as for any other local property.
                s.entries = d.params.mapIndexed { i, e -> scoped { ctx -> property(e, parent!!, "$key/$i", ctx) } }
                s
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
         * The members FIR generates for source data class, data object or
         * value class [d] that it doesn't declare itself: equals, hashCode
         * and toString, which are in its member scope but not among its
         * declarations.
         */
        fun generated(d: KcDecl, c: KcClassSymbol, key: String): List<KaCallableSymbol> {
            if (origin != KaSymbolOrigin.SOURCE || !d.isData && !d.has(KcDecl.VALUE)) return emptyList()
            fun type(id: String, nullable: Boolean = false) = KcType(KcType.CLASS, id, if (nullable) 1 else 0, 0, "", 0, emptyList())
            fun fn(name: String, ret: String, flags: Long, params: List<KcDecl>) = KcDecl(KcDecl.FUNCTION, name, 0, "",
                KcDecl.PUBLIC, KcDecl.OPEN, flags, 0, type(ret), null, emptyList(), emptyList(), emptyList(), emptyList(), params, emptyList())
            val other = KcDecl(KcDecl.VALUE_PARAMETER, "other", 0, "", KcDecl.PUBLIC, KcDecl.FINAL, 0, 0, type("kotlin/Any", true),
                null, emptyList(), emptyList(), emptyList(), emptyList(), emptyList(), emptyList())
            return listOf(
                fn("equals", "kotlin/Boolean", KcDecl.OVERRIDE or KcDecl.OPERATOR, listOf(other)),
                fn("hashCode", "kotlin/Int", 0, emptyList()),
                fn("toString", "kotlin/String", 0, emptyList()),
            ).filter { g ->
                d.members.none { it.kind == KcDecl.FUNCTION && it.name == g.name && it.params.size == g.params.size && it.receiver == null }
            }.map { g -> decl(g, c, "$key/~${g.name}") as KaCallableSymbol }
        }

        /**
         * A property, splitting its annotations by use-site target between
         * the property and its accessors. For a constructor `val`, the model
         * already has only the property's: the untargeted ones that may be
         * on a property are on both it and the parameter, as in Kotlin 2.4.
         */
        fun property(d: KcDecl, parent: KaSymbol, key: String, ctx: KcTypeContext): KaPropertySymbol {
            val psi = psi(d.node)
            check(psi == null && (origin != KaSymbolOrigin.SOURCE || d.node == 0) || psi is KtProperty || psi is KtParameter || psi is KtDestructuringDeclarationEntry) {
                "kc: property ${d.name} in $where is $psi"
            }
            val own = d.annotations.filter {
                when (it.target) {
                    "get", "set", "setparam", "field", "param" -> false
                    else -> !onReceiverType(it, psi)
                }
            }
            // A Java annotation class's method is a synthetic property, at
            // the method. A Java source enum's synthetic entries property is
            // its class, as in FIR; KSP then gives it no accessors.
            val javaAnnotation = (origin == KaSymbolOrigin.JAVA_SOURCE || origin == KaSymbolOrigin.JAVA_LIBRARY) &&
                (parent as? KcClassSymbol)?.info?.decl?.kind == KcDecl.ANNOTATION_CLASS
            val owner = javaOwnerPsi(parent)
            val src = psi ?: if (javaAnnotation) (owner as? PsiClass)?.findMethodsByName(d.name, false)?.firstOrNull() ?: owner else owner
            if (javaAnnotation) return javaAnnotationProperty(d, src, parent, key, own)
            // A Java library enum's entries is its class too, once that has PSI.
            val classFile = if (origin == KaSymbolOrigin.JAVA_LIBRARY && d.name == "entries" && d.has(KcDecl.STATIC) &&
                (parent as? KcClassSymbol)?.classKind == KaClassKind.ENUM_CLASS) { { parent.psi } } else null
            val p = KcPropertySymbol(info(d, src, parent, key, own, classFile))
            register(p, key, src)
            p.typeParams = typeParams(d, p, key)
            ctx.typeParams = p.typeParams
            p.contexts = contexts(d, p, key)
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
                val fieldPsi = (p.info.psi as? KtProperty)?.fieldDeclaration
                p.backingField = KcBackingFieldSymbol(
                    KcInfo(this@KcRound, d, fieldPsi, p, "$key/field", field, types, p.info.origin), p,
                ).also { register(it, "$key/field", fieldPsi) }
            }
            // An accessor's locals are its; the model has them on the property.
            locals(d, key) { psi ->
                listOfNotNull(p.getterSymbol, p.setterSymbol).firstOrNull { a ->
                    val ap = (a as? KcGetterSymbol)?.info?.psi ?: (a as? KcSetterSymbol)?.info?.psi
                    ap != null && psi != null && PsiTreeUtil.isAncestor(ap, psi, true)
                } ?: p
            }
            return p
        }

        /**
         * Java annotation class method [d], at [src], as a synthetic Java
         * property with a getter, whose Java getter is the method.
         */
        fun javaAnnotationProperty(d: KcDecl, src: PsiElement?, parent: KaSymbol, key: String, own: List<KcAnnotation>): KaPropertySymbol {
            info(d, src, parent, key, own)
            val p = KcJavaAnnotationPropertySymbol(KcInfo(this@KcRound, d, src, parent, key, own, types, KaSymbolOrigin.JAVA_SYNTHETIC_PROPERTY))
            register(p, key, src)
            p.getterSymbol = KcGetterSymbol(
                KcInfo(this@KcRound, d, null, p, "$key/get", d.annotations.filter { it.target == "get" }, types, p.info.origin), p,
            ).also { register(it, "$key/get", null) }
            val m = KcDecl(KcDecl.FUNCTION, d.name, 0, "", visibility = d.visibility, modality = d.modality, flags = 0,
                variance = KcDecl.INVARIANT, type = d.type, receiver = null, supertypes = emptyList(), bounds = emptyList(),
                annotations = d.annotations, typeParams = emptyList(), params = emptyList(), members = emptyList())
            p.javaGetter = KcFunctionSymbol(KcInfo(this@KcRound, m, src, parent, "$key/java", own, types, origin)).also {
                it.typeParams = emptyList()
                it.params = emptyList()
                register(it, "$key/java", null)
            }
            return p
        }

        /**
         * The symbols of [d]'s local declarations, whose keys extend
         * [key]; [parent] gives the symbol that contains the one at a psi
         * element.
         */
        fun locals(d: KcDecl, key: String, parent: (PsiElement?) -> KaSymbol) {
            d.locals.forEachIndexed { i, l -> decl(l, parent(psi(l.node)), "$key/!${l.kind}:${l.name}:$i") }
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
                // @all: on a constructor property is its parameter's too.
                val own = v.annotations.filter { it.target == "" || it.target == "param" || it.target == "all" }
                val default = v.default?.let(::value)
                register(KcValueParameterSymbol(info(v, psi, parent, k, own), default), k, psi)
            }

        /** The context parameters of [d], a function or a property, [parent]. */
        fun contexts(d: KcDecl, parent: KaSymbol, key: String): List<KaContextParameterSymbol> =
            d.contexts.mapIndexed { i, v ->
                val k = "$key/ctx$i"
                val psi = psiOf<KtParameter>(v)
                register(KcContextParameterSymbol(info(v, psi, parent, k, v.annotations)), k, psi)
            }
    }

    companion object {
        /** The nodes of a library class: none. */
        @Suppress("UNCHECKED_CAST")
        private val noNodes = arrayOfNulls<ASTNode>(1) as Array<ASTNode>

        /** Whether the PSI join checks each file's kinds; -Dkc.checkKinds=false turns it off, to time it. */
        private val checkKinds = System.getProperty("kc.checkKinds") != "false"

        private val memberKey = Regex("/(?=[0-9<(])")

        /** A fake override's key: its class's key, /~, its index in the member scope, and a member's suffix. */
        private val fakeKey = Regex("^(.*)/~(\\d+)(.*)$")
        private val declaredKey = Regex("^(.*)/~d(\\d+)(.*)$")

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

/**
 * kc's psi tree of a file and kotlin-compiler's differ in shape, so kc's
 * node IDs can't be mapped to its PSI.
 */
internal class KcPsiMismatch(val path: String, kcNodes: Int, psiNodes: Int) :
    IllegalStateException("kc: the PSI of $path differs from kc's tree ($kcNodes nodes in kc, $psiNodes in PSI, or their kinds)")
