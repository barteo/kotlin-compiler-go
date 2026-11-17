package io.github.barteo.kc.aa

import com.intellij.openapi.Disposable
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.vfs.StandardFileSystems
import com.intellij.openapi.vfs.VirtualFileManager
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.psi.KtFile
import java.io.File

/**
 * kc's Analysis API for a client other than KSP: a linter, such as detekt
 * (jvm/detekt's patches), that analyzes one module's Kotlin sources once.
 * Its session answers what kc resolved in the code too ([KcLint]), and a
 * member kc doesn't answer returns an empty value ([KcMissing.lenient]).
 * Nothing here needs KSP on the class path.
 */
object KcStandalone {
    /**
     * Opens a session over [config]'s module, which lasts until
     * [disposable] is disposed. kc analyzes the sources on the first
     * `analyze` call.
     */
    @JvmStatic
    fun open(disposable: Disposable, config: KcModuleConfig): Session {
        KcMissing.lenient = true
        return Session(disposable, config)
    }

    /** A module's session: its IntelliJ project, its files and kc's analysis of them. */
    class Session internal constructor(disposable: Disposable, private val config: KcModuleConfig) {
        private val environment = KcEnvironment("standalone", config)

        init {
            Disposer.register(disposable, Disposable { close() })
        }

        val project: Project get() = environment.project

        // MockProject.registerService, called reflectively: kotlinc can't see
        // MockProject's supertypes in kotlin-compiler's IntelliJ core.
        fun <T : Any> registerProjectService(serviceInterface: Class<T>, serviceImplementation: T) {
            project.javaClass.getMethod("registerService", Class::class.java, Any::class.java)
                .invoke(project, serviceInterface, serviceImplementation)
        }

        fun <T : Any> registerProjectService(serviceImplementation: Class<T>) {
            project.javaClass.getMethod("registerService", Class::class.java).invoke(project, serviceImplementation)
        }

        /** The module's Kotlin files, scripts too, by path. */
        val ktFiles: List<KtFile> by lazy {
            val fs = VirtualFileManager.getInstance().getFileSystem(StandardFileSystems.FILE_PROTOCOL)
            config.sourceRoots.asSequence()
                .flatMap { root -> root.walkTopDown().filter { it.isFile && (it.extension == "kt" || it.extension == "kts") } }
                .map { it.absoluteFile.normalize() }
                .distinct()
                .sortedBy { it.path }
                .mapNotNull { f -> fs.refreshAndFindFileByPath(f.path)?.let(environment::psiFile) as? KtFile }
                .toList()
        }

        private var handle = 0L
        private var model = 0L

        init {
            registerProjectService(org.jetbrains.kotlin.psi.KotlinReferenceProvidersService::class.java, KcReferenceProviders)
            environment.provider.lazySession = lazy(LazyThreadSafetyMode.SYNCHRONIZED) { analyze() }
        }

        /** kc's analysis of the module: its model, the round of it and the facts of its code. */
        private fun analyze(): KaSession {
            val t = KcTimer()
            val start = System.nanoTime()
            // kc analyzes the Kotlin files; it doesn't take scripts.
            val files = ktFiles.filter { !it.isScript() && it.virtualFile != null }
            handle = KcOps.open(
                KcOps.Config(
                    jdkHome = config.jdkHome?.path,
                    noJdk = false,
                    moduleName = config.moduleName,
                    jobs = Runtime.getRuntime().availableProcessors(),
                    classPath = (config.libraries + config.friends).map { it.path }.distinct(),
                    friendPaths = config.friends.map { it.path },
                    facts = true,
                    optIns = config.optIns,
                ),
            )
            val m = KcOps.update(handle, files.map { File(it.virtualFile.path).canonicalPath })
            model = m.handle
            t.mark("update")
            val byPath = files.associateBy { File(it.virtualFile.path).canonicalPath }
            val joined = m.files.map { f -> f to (byPath[f.path] ?: error("kc: no PSI for ${f.path}")) }
            val errors = m.diagnostics
            if (errors.isNotEmpty()) {
                System.err.println("kc: kc can't compile the sources (${errors.size} errors), so the type-resolution rules see no types:")
                errors.take(10).forEach { System.err.println("kc:   $it") }
            }
            val round = KcRound(joined, { id -> KcOps.cls(model, id) }, { fq -> KcOps.pkg(model, fq) }, emptyList(), project, errors)
            val lint = KcLint(round, joined, model, hasFacts = errors.isEmpty())
            t.mark("round")
            t.print("standalone files=${files.size}")
            KcMissing.log("analyzed ${files.size} files in ${(System.nanoTime() - start) / 1_000_000} ms")
            return KcAnalysis.session(KcAnalysis(round, KcModule(project, config), KcLibraryModule(project, config), lint))
        }

        private fun close() {
            environment.provider.clear()
            if (model != 0L) KcOps.close(model)
            if (handle != 0L) KcOps.close(handle)
            environment.dispose()
        }
    }
}
