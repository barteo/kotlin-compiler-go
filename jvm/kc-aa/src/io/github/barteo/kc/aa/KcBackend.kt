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

/** The `kc` KSP backend: the Analysis API answered by kc through libkc. */
class KcBackend : KspBackend {
    override val name: String get() = "kc"

    override fun open(config: KSPConfig, disposable: Disposable): KspBackendSession = KcSession(config, disposable)
}

private class KcSession(private val config: KSPConfig, disposable: Disposable) : KspBackendSession {
    private val timer = KcTimer()

    /** The Java sources the module starts with. */
    private val javaSources: List<String> = ((config as? KSPJvmConfig)?.javaSourceRoots.orEmpty()).distinct().flatMap { root ->
        root.walkTopDown().filter { it.isFile && it.extension == "java" }.sortedBy { it.path }.map { it.canonicalPath }.toList()
    }
    private val env: KotlinCoreEnvironment = run {
        val conf = CompilerConfiguration()
        conf.extensionsStorage = CompilerPluginRegistrar.ExtensionStorage()
        val jvm = config as? KSPJvmConfig
        if (jvm != null) {
            // KSP reads Java sources through Java PSI, which resolves their
            // names (annotation classes, constants) against the class path
            // and the other Java sources, and it reads library class files
            // (checked exceptions, member order, backing fields, Java
            // annotation defaults) through the Java file manager, which
            // must give PSI-backed classes (JavaClassImpl).
            conf.addJvmClasspathRoots((config.libraries + config.friends).distinct())
            jvm.jdkHome?.let { conf.put(JVMConfigurationKeys.JDK_HOME, it) }
            conf.addJavaSourceRoots(jvm.javaSourceRoots)
            conf.put(JVMConfigurationKeys.USE_PSI_CLASS_FILES_READING, true)
        }
        KotlinCoreEnvironment.createForProduction(disposable, conf, EnvironmentConfigFiles.JVM_CONFIG_FILES)
    }
    init {
        timer.mark("environment")
    }
    override val project: Project = env.project
    override val module = KcModule(project, config)
    override val extras: KcExtras = KcExtrasImpl { round }
    private val libraryModule = KcLibraryModule(project, config)
    private val provider = KcSessionProvider(project)
    private val psiManager = PsiManager.getInstance(project)
    private val handle: Long = KcOps.open(
        KcOps.Config(
            jdkHome = (config as? KSPJvmConfig)?.jdkHome?.path,
            moduleName = config.moduleName,
            jobs = Runtime.getRuntime().availableProcessors(),
            classPath = (config.libraries + config.friends).map { it.path }.distinct(),
            friendPaths = config.friends.map { it.path },
        ),
    )

    /** The Kotlin and Java sources the module has, generated ones included. */
    private val paths = LinkedHashSet<String>()
    private lateinit var round: KcRound

    /** The model [round] loads library classes from; the next update closes it. */
    private var modelHandle = 0L

    init {
        // MockProject.registerService, called reflectively: kotlinc can't see
        // MockProject's supertypes in kotlin-compiler's IntelliJ core.
        val register = project.javaClass.getMethod("registerService", Class::class.java, Any::class.java)
        register.invoke(project, KaSessionProvider::class.java, provider)
        register.invoke(project, KotlinModificationTrackerFactory::class.java, KcModificationTrackers)
        (config.sourceRoots + config.commonSourceRoots).distinct().forEach { root ->
            root.walkTopDown().filter { it.isFile && it.extension == "kt" }.sortedBy { it.path }
                .forEach { paths += it.canonicalPath }
        }
        paths += javaSources
        timer.mark("open")
        timer.print("session")
        update()
    }

    private fun update() {
        val t = KcTimer()
        val model = KcOps.update(handle, paths.toList())
        t.mark("update")
        if (System.getProperty("kc.diagnostics") != null) model.diagnostics.forEach { System.err.println("kc: $it") }
        val fs = VirtualFileManager.getInstance().getFileSystem(StandardFileSystems.FILE_PROTOCOL)
        val joined = model.files.map { f ->
            val vf: VirtualFile = fs.refreshAndFindFileByPath(f.path) ?: error("kc: no virtual file for ${f.path}")
            f to (psiManager.findFile(vf) as? KtFile ?: error("kc: ${f.path} is not a Kotlin file"))
        }
        val java = paths.filter { it.endsWith(".java") }.mapNotNull { p ->
            fs.refreshAndFindFileByPath(p)?.let { psiManager.findFile(it) as? PsiJavaFile }
        }
        t.mark("psi")
        if (this::round.isInitialized) {
            round.token.invalidation = "replaced by the next round"
            KcOps.close(modelHandle)
        }
        modelHandle = model.handle
        round = KcRound(joined, { id -> KcOps.cls(model.handle, id) }, { fq -> KcOps.pkg(model.handle, fq) }, java)
        provider.session = KcAnalysis.session(KcAnalysis(round, module, libraryModule))
        t.mark("round")
        t.print("round files=${paths.size}")
    }

    override fun sourceFiles(): List<KSFile> =
        round.fileSymbols.map { KSFileImpl.getCached(it) } + round.javaFiles.map { KSFileJavaImpl.getCached(it) }

    // kc's checker doesn't record its name lookups yet, so KSP rebuilds on
    // every run on this backend (docs/analysis-api.md, "Incremental
    // processing").
    override val recordsLookups: Boolean get() = false

    override fun beginRound(lookupTracker: DualLookupTracker) {}

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
        if (this::round.isInitialized) {
            round.token.invalidation = "session closed"
            KcOps.close(modelHandle)
        }
        KcOps.close(handle)
    }
}

/**
 * Trackers that never change: KSP's KotlinAlwaysAccessibleLifetimeTokens
 * (type projections it builds, synthetic annotations) watch the source
 * tracker. kc's own lifetime token ends with the round instead.
 */
private object KcModificationTrackers : KotlinModificationTrackerFactory {
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
