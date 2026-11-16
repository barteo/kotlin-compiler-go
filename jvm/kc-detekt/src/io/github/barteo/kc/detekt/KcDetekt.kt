package io.github.barteo.kc.detekt

import com.intellij.core.CoreApplicationEnvironment
import com.intellij.openapi.Disposable
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.StandardFileSystems
import com.intellij.openapi.vfs.VirtualFileManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.PsiManager
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.projectStructure.KaLibraryModule
import org.jetbrains.kotlin.analysis.api.projectStructure.KaModule
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.analysis.api.session.KaSessionProvider
import org.jetbrains.kotlin.cli.extensionsStorage
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar
import org.jetbrains.kotlin.config.AnalysisFlags
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.config.LanguageVersionSettings
import org.jetbrains.kotlin.platform.TargetPlatform
import org.jetbrains.kotlin.psi.KtElement
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KotlinReferenceProvidersService
import java.io.File
import java.nio.file.Path

/**
 * detekt on kc: the standalone Analysis API session that detekt builds is
 * answered by kc. The PSI is kotlin-compiler's, which detekt's rules walk;
 * kc parses the same files with its own psi parser, checks them as one
 * module, and the session reads its facts, keyed by psi node, through
 * libkc.
 */
object KcDetekt {
    /** Whether -Dkc.strict is set: a member kc doesn't answer throws instead of returning an empty value. */
    internal val strict = System.getProperty("kc.strict") == "true"

    /** Whether -Dkc.trace is set: prints timings and what kc doesn't answer. */
    internal val trace = System.getProperty("kc.trace") == "true"

    internal fun log(msg: String) {
        if (trace) System.err.println("kc: $msg")
    }

    /** The IntelliJ environment of a session: a core environment that only parses. */
    class Environment(private val disposable: Disposable) {
        private val core: KotlinCoreEnvironment = run {
            val conf = CompilerConfiguration()
            conf.extensionsStorage = CompilerPluginRegistrar.ExtensionStorage()
            KotlinCoreEnvironment.createForProduction(disposable, conf, EnvironmentConfigFiles.JVM_CONFIG_FILES)
        }

        val project: Project get() = core.project

        val applicationEnvironment: CoreApplicationEnvironment get() = core.projectEnvironment.environment

        private var source: KcSourceModule? = null

        // MockProject.registerService, called reflectively: kotlinc can't see
        // MockProject's supertypes in kotlin-compiler's IntelliJ core.
        fun registerService(iface: Class<*>, impl: Any) {
            project.javaClass.getMethod("registerService", Class::class.java, Any::class.java).invoke(project, iface, impl)
        }

        fun registerService(impl: Class<*>) {
            project.javaClass.getMethod("registerService", Class::class.java).invoke(project, impl)
        }

        /** Starts the session of the source module among [modules]. */
        fun start(modules: List<KaModule>) {
            val src = modules.filterIsInstance<KcSourceModule>().singleOrNull()
                ?: error("kc: detekt on kc analyzes one source module, not ${modules.size}")
            source = src
            val psi = PsiManager.getInstance(project)
            val fs = VirtualFileManager.getInstance().getFileSystem(StandardFileSystems.FILE_PROTOCOL)
            val files = src.roots.asSequence()
                .flatMap { root -> root.toFile().walkTopDown().filter { it.isFile && (it.extension == "kt" || it.extension == "kts") } }
                .map { it.absoluteFile.normalize() }
                .distinct()
                .sortedBy { it.path }
                .mapNotNull { fs.findFileByPath(it.path)?.let(psi::findFile) as? KtFile }
                .toList()
            src.files = files
            val libraries = src.dependencies.filterIsInstance<KcLibraryModule>()
            val jdk = libraries.firstNotNullOfOrNull { it.jdkHome }
            val friendRoots = src.friends.filterIsInstance<KcLibraryModule>().flatMap { it.roots }.map { it.toString() }
            val cfg = KcLintOps.Config(
                jdkHome = jdk?.toString(),
                moduleName = src.name,
                jobs = Runtime.getRuntime().availableProcessors(),
                classPath = (libraries.flatMap { it.roots }.map { it.toString() } + friendRoots).distinct(),
                friendPaths = friendRoots.distinct(),
                // detekt's -opt-in compiler arguments.
                // and -Dkc.optIns, comma-separated, for a detekt-cli that can't pass them.
                optIns = src.languageVersionSettings.getFlag(AnalysisFlags.optIn) +
                    System.getProperty("kc.optIns").orEmpty().split(',').filter { it.isNotBlank() },
            )
            val session = KcSession(project, files, cfg)
            registerService(KaSessionProvider::class.java, KcSessionProvider(project, session))
            // A name's mainReference, which the Analysis API's own session registers.
            registerService(KotlinReferenceProvidersService::class.java, KcReferenceProviders())
        }

        fun modulesWithFiles(): Map<KaSourceModule, List<PsiFile>> =
            source?.let { mapOf<KaSourceModule, List<PsiFile>>(it to it.files) } ?: emptyMap()
    }
}

/** The lifetime of a session's symbols: as long as the session. */
internal object KcToken : KaLifetimeToken() {
    override fun isValid() = true
    override fun getInvalidationReason() = "valid"
    override fun isAccessible() = true
    override fun getInaccessibilityReason() = "accessible"
}

/** Hands out the session for every `analyze` call. */
internal class KcSessionProvider(project: Project, private val session: KcSession) : KaSessionProvider(project) {
    private val kaSession: KaSession by lazy { KcAnalysis.session(KcAnalysis(session)) }
    override fun getAnalysisSession(useSiteElement: KtElement): KaSession = kaSession
    override fun getAnalysisSession(useSiteModule: KaModule): KaSession = kaSession
    override fun beforeEnteringAnalysis(session: KaSession, useSiteElement: KtElement) {}
    override fun beforeEnteringAnalysis(session: KaSession, useSiteModule: KaModule) {}
    override fun afterLeavingAnalysis(session: KaSession, useSiteElement: KtElement) {}
    override fun afterLeavingAnalysis(session: KaSession, useSiteModule: KaModule) {}
    override fun handleAnalysisException(throwable: Throwable, session: KaSession, useSiteElement: KtElement): Nothing =
        throw throwable
    override fun handleAnalysisException(throwable: Throwable, session: KaSession, useSiteModule: KaModule): Nothing =
        throw throwable
    override fun clearCaches() {}
}

/** The module of the sources. */
class KcSourceModule internal constructor(
    override val project: Project,
    override val name: String,
    internal val roots: List<Path>,
    internal val dependencies: List<KaModule>,
    internal val friends: List<KaModule>,
    override val languageVersionSettings: LanguageVersionSettings,
    override val targetPlatform: TargetPlatform,
) : StubKaSourceModule() {
    /** The module's files, once the session has started. */
    internal var files: List<KtFile> = emptyList()

    override val psiRoots: List<PsiFile> get() = files
    override val directRegularDependencies: List<KaModule> get() = dependencies
    override val directDependsOnDependencies: List<KaModule> get() = emptyList()
    override val directFriendDependencies: List<KaModule> get() = friends
    override val transitiveDependsOnDependencies: List<KaModule> get() = emptyList()
    override fun toString() = "KcSourceModule($name)"
}

/** A library or the JDK: binary roots kc puts on the class path. */
class KcLibraryModule internal constructor(
    override val project: Project,
    override val libraryName: String,
    internal val roots: List<Path>,
    override val isSdk: Boolean,
    internal val jdkHome: Path?,
    override val targetPlatform: TargetPlatform,
) : StubKaLibraryModule() {
    override val binaryRoots: Collection<Path> get() = roots
    override val librarySources: org.jetbrains.kotlin.analysis.api.projectStructure.KaLibrarySourceModule? get() = null
    override val directRegularDependencies: List<KaModule> get() = emptyList()
    override val directDependsOnDependencies: List<KaModule> get() = emptyList()
    override val directFriendDependencies: List<KaModule> get() = emptyList()
    override val transitiveDependsOnDependencies: List<KaModule> get() = emptyList()
    override fun toString() = "KcLibraryModule($libraryName)"
}

internal fun File.canonical(): String = runCatching { canonicalPath }.getOrDefault(absolutePath)
