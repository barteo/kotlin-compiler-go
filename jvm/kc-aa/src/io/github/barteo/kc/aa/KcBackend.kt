package io.github.barteo.kc.aa

import com.google.devtools.ksp.impl.DualLookupTracker
import com.google.devtools.ksp.impl.backend.KcExtras
import com.google.devtools.ksp.impl.backend.KspBackend
import com.google.devtools.ksp.impl.backend.KspBackendSession
import com.google.devtools.ksp.impl.symbol.kotlin.KSFileImpl
import com.google.devtools.ksp.impl.symbol.kotlin.KSFileJavaImpl
import com.google.devtools.ksp.processing.KSPConfig
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.google.devtools.ksp.symbol.KSFile
import com.intellij.openapi.Disposable
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.ModificationTracker
import com.intellij.openapi.vfs.StandardFileSystems
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.openapi.vfs.VirtualFileManager
import com.intellij.psi.PsiAnnotation
import com.intellij.psi.PsiJavaFile
import com.intellij.psi.PsiManager
import org.jetbrains.kotlin.analysis.api.types.KaClassType
import org.jetbrains.kotlin.incremental.components.Position
import org.jetbrains.kotlin.incremental.components.ScopeKind
import org.jetbrains.kotlin.analysis.api.types.KaFlexibleType
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.analysis.api.platform.modification.KotlinModificationTrackerFactory
import org.jetbrains.kotlin.analysis.api.projectStructure.KaModule
import org.jetbrains.kotlin.analysis.api.session.KaSessionProvider
import org.jetbrains.kotlin.analysis.api.symbols.KaCallableSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaClassSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaKotlinPropertySymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaSymbol
import org.jetbrains.kotlin.analysis.api.symbols.KaValueParameterSymbol
import org.jetbrains.kotlin.analysis.api.symbols.typeParameters
import org.jetbrains.kotlin.analysis.api.types.KaFunctionType
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment
import org.jetbrains.kotlin.cli.jvm.config.addJavaSourceRoots
import org.jetbrains.kotlin.cli.jvm.config.addJvmClasspathRoots
import org.jetbrains.kotlin.config.JVMConfigurationKeys
import org.jetbrains.kotlin.load.kotlin.KotlinBinaryClassCache
import org.jetbrains.kotlin.load.kotlin.KotlinClassFinder
import org.jetbrains.kotlin.metadata.deserialization.MetadataVersion
import org.jetbrains.kotlin.cli.extensionsStorage
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.config.JvmTarget
import org.jetbrains.kotlin.config.LanguageVersionSettings
import org.jetbrains.kotlin.config.LanguageVersionSettingsImpl
import org.jetbrains.kotlin.fileClasses.JvmFileClassUtil
import org.jetbrains.kotlin.load.java.structure.JavaAnnotationArgument
import org.jetbrains.kotlin.load.kotlin.KotlinJvmBinaryClass
import org.jetbrains.kotlin.load.kotlin.TypeMappingMode
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.platform.TargetPlatform
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.psi.KtFile
import java.io.File

/**
 * Each source's canonical path, by its path, kept by the process: a
 * daemon's runs walk the same sources again, and resolving each path's
 * links is a good part of a warm run's setup.
 */
private val canonicalPaths = java.util.concurrent.ConcurrentHashMap<String, String>()

private fun canonicalPath(f: File): String = canonicalPaths.getOrPut(f.absolutePath) { f.canonicalPath }

/**
 * The files under source roots, kept by the process: a daemon's runs list
 * the same directories again. Each directory's listing is kept with the
 * directory's modification time, which adding, removing or renaming an
 * entry changes (editing a file doesn't, and needn't), so a later walk
 * stats the directories, not each file. As libkc's read cache does with
 * files (analysis/ka/readcache.go), a directory modified within
 * [RACY_MILLIS] of being listed is listed again until it is older: a
 * coarse clock may not have moved since.
 */
internal object SourceWalk {
    private class Listing(val modified: java.nio.file.attribute.FileTime, val listedAt: Long, val files: List<File>, val dirs: List<File>)

    private val listings = java.util.concurrent.ConcurrentHashMap<String, Listing>()
    private const val RACY_MILLIS = 2000L

    /** The files under [root] named *.[ext], sorted by path: `root.walkTopDown()`'s, as files. */
    fun files(root: File, ext: String): List<File> {
        val out = ArrayList<File>()
        fun walk(dir: File) {
            val l = listing(dir) ?: return
            l.files.filterTo(out) { it.extension == ext }
            l.dirs.forEach(::walk)
        }
        if (root.isFile) {
            if (root.extension == ext) out += root
        } else {
            walk(root)
        }
        out.sortBy { it.path }
        return out
    }

    private fun listing(dir: File): Listing? {
        val now = System.currentTimeMillis()
        val modified = try {
            java.nio.file.Files.getLastModifiedTime(dir.toPath())
        } catch (e: java.io.IOException) {
            listings.remove(dir.path)
            return null
        }
        listings[dir.path]?.let { l ->
            if (l.modified == modified && modified.toMillis() < l.listedAt - RACY_MILLIS) return l
        }
        val entries = dir.listFiles() ?: return null
        val files = ArrayList<File>()
        val dirs = ArrayList<File>()
        for (e in entries) {
            if (e.isDirectory) dirs += e else if (e.isFile) files += e
        }
        return Listing(modified, now, files, dirs).also { listings[dir.path] = it }
    }
}

private val noParseAhead = !System.getenv("KC_NO_PARSE_AHEAD").isNullOrEmpty()

/** KC_NO_JAVA_DECLS=1 asks libkc for the Java sources' classes, and the names it has no class of, every round. */
private val noJavaDecls = System.getenv("KC_NO_JAVA_DECLS") == "1"

// Only the files Gradle says were modified are parsed while libkc updates,
// as before step 59, not the new ones (for comparison).
private val noAheadNew = System.getenv("KC_NO_AHEAD_NEW") == "1"

/** The `kc` KSP backend: the Analysis API answered by kc through libkc. */
class KcBackend : KspBackend {
    override val name: String get() = "kc"

    override fun open(config: KSPConfig, disposable: Disposable): KspBackendSession = KcSession(config, disposable)
}

private class KcSession(private val config: KSPConfig, @Suppress("UNUSED_PARAMETER") disposable: Disposable) : KspBackendSession {
    private val timer = KcTimer()

    /** The Java sources the module starts with. */
    private val javaSources: List<String> = ((config as? KSPJvmConfig)?.javaSourceRoots.orEmpty()).distinct().flatMap { root ->
        SourceWalk.files(root, "java").map(::canonicalPath)
    }
    private val environment = KcEnvironments.acquire(config, javaSources)
    private val env: KotlinCoreEnvironment = environment.env
    init {
        timer.mark("environment")
    }
    override val project: Project = env.project
    override val module = KcModule(project, config)
    override val extras: KcExtras = KcExtrasImpl { round }
    private val libraryModule = KcLibraryModule(project, config)
    private val provider = environment.provider
    /**
     * The class-path classes libkc modeled, by outermost class ID, kept by
     * the environment for the rounds after: its class path doesn't change,
     * so a round asks libkc only for those none has yet, and not again for
     * a name libkc said it has no class of. The Java sources' classes are
     * kept by the model's KcModel.java, which libkc gives a round whose
     * Java sources, and the Kotlin classes they name, are those of one
     * before; a class a Java source declares, which hides the class
     * path's, isn't kept with those.
     */
    private val libraryDecls = environment.libraryDeclsOf(
        "module=${config.moduleName}\n" + config.friends.joinToString("\n") { it.path })
    private val handle: Long = KcOps.open(
        KcOps.Config(
            jdkHome = (config as? KSPJvmConfig)?.jdkHome?.path,
            moduleName = config.moduleName,
            jobs = Runtime.getRuntime().availableProcessors(),
            classPath = (config.libraries + config.friends).map { it.path }.distinct(),
            friendPaths = config.friends.map { it.path },
            // KSP only adds files between its rounds.
            fixedSources = true,
        ),
    )

    /** The Kotlin and Java sources the module has, generated ones included. */
    private val paths = LinkedHashSet<String>()
    private lateinit var round: KcRound

    init {
        (config.sourceRoots + config.commonSourceRoots).distinct().forEach { root ->
            SourceWalk.files(root, "kt").forEach { paths += canonicalPath(it) }
        }
        paths += javaSources
        timer.mark("open")
        timer.print("session")
        update(config.modifiedSources.filter { it.path.endsWith(".kt") }.map(::canonicalPath).filter { it in paths })
    }

    /**
     * Updates libkc's model to [paths] and builds the round over it. While
     * libkc updates, the PSI the round would make and parse after it is
     * made and parsed (KcEnvironment.parseAhead), unless KC_NO_PARSE_AHEAD
     * is set: that of [modified], the sources Gradle says changed, and of
     * the sources the environment hasn't parsed yet, the Kotlin files no
     * round joined and the Java files it never loaded. That is every file
     * in a new environment's first round, and the generated files in the
     * rounds after; a warm run's others are joined and loaded already.
     */
    private fun update(modified: List<String> = emptyList()) {
        val t = KcTimer()
        val (reused, reparsed) = environment.reused to environment.reparsed
        val checks = environment.reusableChecks()
        val base = environment.baseFiles()
        val toParse = if (noAheadNew) modified else (modified + paths.filter {
            if (it.endsWith(".kt")) !environment.trees.has(it) else !environment.loaded(it)
        }).distinct()
        val ahead = if (toParse.isEmpty() || noParseAhead) null else environment.parseAhead(toParse)
        val model = try {
            KcOps.update(handle, paths.toList(), checks, base)
        } finally {
            if (ahead != null) {
                t.mark("native")
                ahead.join()
                t.mark("wait")
            }
        }
        t.mark("update")
        val next = if (model.same) {
            // The check of a round the environment kept: its sources, and
            // its PSI, are the same, so the round is too.
            KcOps.close(model.handle)
            val kept = environment.reuse(model.check)
            printDiagnostics(kept.diagnostics)
            kept.round.also { it.token.invalidation = null }
        } else {
            printDiagnostics(model.diagnostics)
            val built = try {
                build(model, t)
            } catch (e: Throwable) {
                KcOps.close(model.handle)
                throw e
            }
            t.mark("build")
            built.also { environment.keep(model.check, it, model.handle, model.diagnostics, model.files) }
        }
        val touched = if (this::round.isInitialized) " previous-touched(${round.touched()})" else ""
        if (this::round.isInitialized && round !== next) round.token.invalidation = "replaced by the next round"
        round = next
        provider.session = KcAnalysis.session(KcAnalysis(round, module, libraryModule))
        t.mark("round")
        t.print("round files=${paths.size} same=${model.same} psi-reused=${environment.reused - reused} psi-reparsed=${environment.reparsed - reparsed} joined=${next.joinedNew.get()}$touched")
    }

    private fun printDiagnostics(ds: List<KcDiagnostic>) {
        if (System.getProperty("kc.diagnostics") != null) ds.forEach { System.err.println("kc: $it") }
    }


    /** The class [id] (outermost) of [model]'s class path or Java sources, or null. */
    private fun libraryDecl(model: KcModel, javaClasses: Set<String>, id: String): KcDecl? {
        if (id in javaClasses) {
            val java = if (model.java == 0L || noJavaDecls) null else libraryDecls.java(model.java)
            java?.let { synchronized(libraryDecls) { if (id in it) return it[id] } }
            return KcOps.cls(model.handle, id).first.also { d -> java?.let { synchronized(libraryDecls) { it[id] = d } } }
        }
        synchronized(libraryDecls) {
            libraryDecls.decls[id]?.let { return it }
            if (id in libraryDecls.absent) return null
        }
        val (d, absent) = KcOps.cls(model.handle, id)
        synchronized(libraryDecls) {
            if (d != null && !d.has(KcDecl.JAVA_SOURCE)) libraryDecls.decls[id] = d
            if (absent && !noJavaDecls) libraryDecls.absent += id
        }
        return d
    }

    /** The round of [model], whose sources' PSI it joins. */
    private fun build(model: KcModel, t: KcTimer): KcRound {
        val fs = VirtualFileManager.getInstance().getFileSystem(StandardFileSystems.FILE_PROTOCOL)
        val joined = model.files.map { f ->
            f to (environment.psiFile(f.path, f.text) as? KtFile ?: error("kc: no Kotlin PSI for ${f.path}"))
        }
        val java = paths.filter { it.endsWith(".java") }.mapNotNull { p ->
            fs.refreshAndFindFileByPath(p)?.let { environment.psiFile(it) as? PsiJavaFile }
        }
        val javaClasses = HashSet<String>()
        for (f in java) {
            val pkg = f.packageName.replace('.', '/')
            for (c in f.classes) c.name?.let { javaClasses += if (pkg.isEmpty()) it else "$pkg/$it" }
        }
        t.mark("psi")
        return KcRound(joined, { id -> libraryDecl(model, javaClasses, id) }, { fq -> KcOps.pkg(model.handle, fq) }, java, project, environment.trees,
            if (this::round.isInitialized) round.sizes() else KcRound.Sizes(), t)
    }

    override fun sourceFiles(): List<KSFile> =
        round.fileSymbols.map { KSFileImpl.getCached(it) } + round.javaFiles.map { KSFileJavaImpl.getCached(it) }

    // kc's checker records the names each file looks up (KcFile.lookups);
    // they are replayed into KSP's tracker for the files it hasn't seen yet,
    // under the paths KSFile.filePath gives. All are recorded as PACKAGE
    // lookups, class scopes too, as FIR's lookup tracker component does: KSP
    // finds the files a changed class member dirties in its symbol lookups,
    // and its class tracker, for CLASSIFIER lookups, stays empty on FIR
    // (docs/analysis-api.md, "Step 16: incremental lookups"). Only the
    // dirty files' are: KSP's caches keep the others' lookups, and replace
    // only the dirty files' with what the tracker has. Without incremental
    // processing KSP keeps none.
    override val recordsLookups: Boolean get() = true

    private val replayed = HashSet<String>()

    override fun beginRound(lookupTracker: DualLookupTracker, dirtyFiles: List<KSFile>) {
        if (!config.incremental) return
        val dirty = dirtyFiles.mapTo(HashSet()) { it.filePath }
        for (s in round.fileSymbols) {
            val path = s.psi.virtualFile.path
            if (path !in dirty || !replayed.add(s.file.path)) continue
            for (l in s.file.lookups)
                lookupTracker.record(path, Position.NO_POSITION, l.scope, ScopeKind.PACKAGE, l.name)
        }
    }

    // Sources aren't edited between rounds (KSP only adds files), and the
    // next update replaces the round, so there is nothing to drop. (Dropping
    // PsiManager's caches would need IntelliJ extension points the core
    // environment doesn't register.)
    override fun dropCaches() {}

    override fun addFiles(kotlinFiles: List<File>, javaFiles: List<File>): List<KSFile> {
        val added = (kotlinFiles + javaFiles).map { it.canonicalPath }.filter { paths.add(it) }.toSet()
        // KSP doesn't edit sources between rounds, so a round that
        // generated no sources keeps this one.
        if (added.isEmpty()) return emptyList()
        update()
        return round.fileSymbols.filter { it.file.path in added }.map { KSFileImpl.getCached(it) } +
            round.javaFiles.filter { it.virtualFile.canonicalPath in added }.map { KSFileJavaImpl.getCached(it) }
    }

    override fun close() {
        // The round's model is the environment's, which keeps the round.
        if (this::round.isInitialized) round.token.invalidation = "session closed"
        KcTimer.printCalls()
        KcOps.close(handle)
        KcEnvironments.release(environment)
    }
}

/**
 * Trackers that never change: KSP's KotlinAlwaysAccessibleLifetimeTokens
 * (type projections it builds, synthetic annotations) watch the source
 * tracker. kc's own lifetime token ends with the round instead.
 */
internal object KcModificationTrackers : KotlinModificationTrackerFactory {
    override fun createProjectWideSourceModificationTracker(): ModificationTracker = ModificationTracker.NEVER_CHANGED
    override fun createProjectWideLibraryModificationTracker(): ModificationTracker = ModificationTracker.NEVER_CHANGED
}

/** The module KSP processes. */
internal class KcModule(override val project: Project, private val config: KSPConfig) : StubKaSourceModule() {
    override val name: String get() = config.moduleName
    override val targetPlatform: TargetPlatform
        get() = (config as? KSPJvmConfig)?.jvmTarget?.let(JvmTarget::fromString)
            ?.let(JvmPlatforms::jvmPlatformByTargetVersion) ?: JvmPlatforms.defaultJvmPlatform
    override val languageVersionSettings: LanguageVersionSettings get() = LanguageVersionSettingsImpl.DEFAULT
    override val directRegularDependencies: List<KaModule> get() = emptyList()
    override val directDependsOnDependencies: List<KaModule> get() = emptyList()
    override val directFriendDependencies: List<KaModule> get() = emptyList()
    override val transitiveDependsOnDependencies: List<KaModule> get() = emptyList()
    override fun toString() = "KcModule($name)"
}

/**
 * The module of every library symbol: kc puts the whole class path, JDK
 * included, in one.
 */
internal class KcLibraryModule(override val project: Project, private val config: KSPConfig) : StubKaLibraryModule() {
    override val libraryName: String get() = "kc-libraries"
    override val binaryRoots: Collection<java.nio.file.Path> get() = config.libraries.map { it.toPath() }
    override val isSdk: Boolean get() = false
    override val librarySources get() = null
    override val targetPlatform: TargetPlatform get() = JvmPlatforms.defaultJvmPlatform
    override val directRegularDependencies: List<KaModule> get() = emptyList()
    override val directDependsOnDependencies: List<KaModule> get() = emptyList()
    override val directFriendDependencies: List<KaModule> get() = emptyList()
    override val transitiveDependsOnDependencies: List<KaModule> get() = emptyList()
    override fun toString() = "KcLibraryModule"
}

/** The FIR-only facts, as kc has them. */
private class KcExtrasImpl(private val round: () -> KcRound) : KcExtras {
    override fun javaAnnotationClass(annotation: PsiAnnotation): ClassId? = round().javaAnnotationClass(annotation)

    override fun javaEnumEntry(reference: com.intellij.psi.PsiReferenceExpression): org.jetbrains.kotlin.name.CallableId? =
        round().javaEnumEntry(reference)

    override fun javaConstant(reference: com.intellij.psi.PsiReferenceExpression): Any? = round().javaConstant(reference)

    override fun optimalModeForReturnType(type: KaType, isAnnotationMethod: Boolean): TypeMappingMode =
        if (isAnnotationMethod) TypeMappingMode.VALUE_FOR_ANNOTATION else optimalMode(type, false)
    override fun optimalModeForValueParameter(type: KaType): TypeMappingMode = optimalMode(type, true)

    /**
     * Kotlin's getOptimalModeForSignaturePart (TypeMappingModeExtensions.kt). A generic value
     * class's underlying type isn't looked through, unlike there.
     */
    @OptIn(org.jetbrains.kotlin.load.kotlin.TypeMappingModeInternals::class)
    private fun optimalMode(type: KaType, canBeUsedInSupertypePosition: Boolean): TypeMappingMode {
        val t = (type as? KaFlexibleType)?.lowerBound ?: type
        if (t !is KaClassType || t.typeArguments.isEmpty()) return TypeMappingMode.DEFAULT
        val isInline = (round().classSymbol(t.classId) as? KcClassSymbol)?.isInline == true
        return TypeMappingMode(
            needInlineClassWrapping = !isInline,
            skipDeclarationSiteWildcards = !canBeUsedInSupertypePosition,
            skipDeclarationSiteWildcardsIfPossible = true,
            genericContravariantArgumentMode =
                if (canBeUsedInSupertypePosition) null else TypeMappingMode(skipDeclarationSiteWildcardsIfPossible = true),
            genericInvariantArgumentMode = if (canBeUsedInSupertypePosition) optimalMode(type, false) else null,
        )
    }
    /**
     * Whether [type] is a Java raw type, `List` for `List<E>`: the model
     * writes it as a platform class type without arguments.
     */
    override fun isRawType(type: KaType): Boolean {
        val t = (type as? KcFlexibleType)?.lowerBound as? KcClassType ?: return false
        return t.typeArguments.isEmpty() && (t.symbol as? KaClassSymbol)?.typeParameters?.isNotEmpty() == true
    }
    override fun abbreviatedTypeAlias(type: KaFunctionType): ClassId? = (type as? KcFunctionType)?.abbrev?.classId

    /**
     * The class that declares a top-level callable on the JVM: a library
     * one's file facade or multi-file part, as FIR's JvmPackagePartSource
     * has it; a source one's file facade (FIR has no part source for
     * sources, and KSP asks only when the light facade class is missing).
     */
    override fun packagePartClass(symbol: KaCallableSymbol): ClassId? {
        val info = when (val s = fakeOverrideOriginal(symbol)) {
            is KcFunctionSymbol -> s.info
            is KcPropertySymbol -> s.info
            is KcGetterSymbol -> s.property.info
            is KcSetterSymbol -> s.property.info
            else -> return null
        }
        return when (val parent = info.parent) {
            is KcPackageSymbol -> info.decl.classId.takeIf { it.isNotEmpty() }?.let(ClassId::fromString)
            is KcFileSymbol -> ClassId.topLevel(fileFacadeClass(parent.psi))
            else -> null
        }
    }
    override fun fileFacadeClass(file: KtFile): FqName = JvmFileClassUtil.getFileClassInfoNoResolve(file).facadeClassFqName
    // Null: KSP then asks the symbol's hasBackingField, which the model
    // answers (FlagBackingField), not KSP's class file cache.
    override fun binaryContainerClass(symbol: KaKotlinPropertySymbol): ClassId? = null
    /**
     * A library annotation parameter's default, from the model rather than
     * from [default] (kc gives KSP no Java class finder). Like FIR, it makes
     * up an empty array or null for a parameter without one.
     */
    override fun javaAnnotationDefault(symbol: KaValueParameterSymbol, default: JavaAnnotationArgument?): KaAnnotationValue? {
        val p = symbol as? KcValueParameterSymbol ?: KcAnalysis.unsupported("KcExtras.javaAnnotationDefault of $symbol")
        p.default?.let { return it }
        val token = p.info.round.token
        val t = p.info.decl.type
        return if (t != null && t.kind == KcType.CLASS && arrayClass.matches(t.classId)) {
            KcArrayValue(emptyList(), null, token)
        } else {
            // FIR's converter gives no value for a non-array parameter
            // without a default, so KSP lists no argument for it.
            null
        }
    }

    private val arrayClass = Regex("kotlin/(Array|[A-Z][a-z]+Array|U[A-Z][a-z]+Array)")
    override fun resolveTypes(symbol: KaSymbol) {}
    /**
     * The module name in the metadata of [symbol]'s library class or file
     * facade, found by walking out from it; null for "main", where KSP
     * then uses "main".
     */
    override fun libraryModuleName(symbol: KaSymbol): String? {
        var s: KaSymbol? = symbol
        while (s != null) {
            val info = when (s) {
                is KcFakeOverride -> { s = s.owner; continue }
                is KcGetterSymbol -> s.property.info
                is KcSetterSymbol -> s.property.info
                is KcClassSymbol -> s.info
                is KcFunctionSymbol -> s.info
                is KcConstructorSymbol -> s.info
                is KcValueParameterSymbol -> s.info
                is KcTypeParameterSymbol -> s.info
                is KcPropertySymbol -> s.info
                is KcEnumEntrySymbol -> s.info
                is KcTypeAliasSymbol -> s.info
                is KcBackingFieldSymbol -> s.info
                is KcLocalVariableSymbol -> s.info
                else -> return null
            }
            if (info.decl.module.isNotEmpty()) return info.decl.module
            s = info.parent
        }
        return null
    }
    /** The Kotlin view of a library class file, which KSP reads for its members' order. */
    override fun kotlinBinaryClass(file: VirtualFile): KotlinJvmBinaryClass? =
        (KotlinBinaryClassCache.getKotlinBinaryClassOrClassFileContent(file, MetadataVersion.INSTANCE)
            as? KotlinClassFinder.Result.KotlinClass)?.kotlinJvmBinaryClass
}
