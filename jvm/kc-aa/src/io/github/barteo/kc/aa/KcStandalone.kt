package io.github.barteo.kc.aa

import com.intellij.openapi.Disposable
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.vfs.StandardFileSystems
import com.intellij.openapi.vfs.VirtualFileManager
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.psi.KtFile
import java.io.File
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CompletionException
import java.util.concurrent.Executors

/**
 * kc's Analysis API for a client other than KSP: a linter, such as detekt
 * (jvm/detekt's patches), that analyzes one module's Kotlin sources once.
 * Its session answers what kc resolved in the code too ([KcLint]), and a
 * member kc doesn't answer returns an empty value ([KcMissing.lenient]).
 * Nothing here needs KSP on the class path.
 */
object KcStandalone {
    /** Whether kc's check starts before the session's environment is built (`-Dkc.prestart`, default true). */
    private val prestart = System.getProperty("kc.prestart") != "false"

    /**
     * Opens a session over [config]'s module, which lasts until
     * [disposable] is disposed. kc starts analyzing the sources right
     * away, in the background; the first `analyze` call waits for it.
     */
    @JvmStatic
    fun open(disposable: Disposable, config: KcModuleConfig): Session {
        KcMissing.lenient = true
        return Session(disposable, config)
    }

    /** A module's session: its IntelliJ project, its files and kc's analysis of them. */
    class Session internal constructor(disposable: Disposable, private val config: KcModuleConfig) {
        // Declared first: the check sets them from its thread, maybe before
        // the initializers below run.
        @Volatile private var handle = 0L
        @Volatile private var model = 0L
        private val start = System.nanoTime()

        /**
         * kc's check of the module, which reads the sources from their
         * paths and needs no PSI: started before anything else, it runs
         * while the session builds its IntelliJ environment
         * (`KotlinCoreEnvironment` and the class path's index), and then
         * the linter's own setup. `-Dkc.prestart=false` starts it after
         * the environment, as before.
         */
        private val checked = CompletableFuture<KcModel>()

        init {
            if (prestart) startCheck()
        }

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

        /**
         * kc's analysis of the module: the check, from the moment the
         * session is opened, while a small pool parses the PSI for the
         * round's index of it. As soon as kc's check is done, the pool
         * decodes each file's facts, off the threads that run the rules.
         */
        private val analysis = CompletableFuture<KaSession>()

        init {
            registerProjectService(org.jetbrains.kotlin.psi.KotlinReferenceProvidersService::class.java, KcReferenceProviders)
            environment.provider.lazySession = lazy(LazyThreadSafetyMode.SYNCHRONIZED) {
                val waitStart = System.nanoTime()
                try {
                    analysis.join()
                } catch (e: CompletionException) {
                    throw e.cause ?: e
                } finally {
                    KcStats.time("the first analyze call waited for kc's analysis", System.nanoTime() - waitStart)
                }
            }
            if (!prestart) startCheck()
            Thread(null, {
                try {
                    analysis.complete(analyze())
                } catch (e: Throwable) {
                    analysis.completeExceptionally(e)
                }
            }, "kc-analysis", 16L shl 20).apply { isDaemon = true }.start()
        }

        /** Starts kc's check of the module's sources, on a thread of its own. */
        private fun startCheck() {
            // A big stack: Go's runtime runs its calls on this thread's.
            Thread(null, {
                try {
                    checked.complete(check())
                } catch (e: Throwable) {
                    checked.completeExceptionally(e)
                }
            }, "kc-check", 64L shl 20).apply { isDaemon = true }.start()
        }

        /** kc's check of the sources, from their paths: the model, and the facts of its code. */
        private fun check(): KcModel {
            val t = KcTimer()
            if (KcMissing.trace) {
                val jvmStart = ProcessHandle.current().info().startInstant().orElse(null)
                if (jvmStart != null) KcStats.time("from the JVM's start to kc's check", java.time.Duration.between(jvmStart, java.time.Instant.now()).toNanos())
            }
            val paths = config.sourceRoots.asSequence()
                .flatMap { root -> root.walkTopDown().filter { it.isFile && it.extension == "kt" } }
                .map { it.canonicalPath }
                .distinct()
                .sorted()
                .toList()
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
            t.mark("open")
            val m = KcOps.update(handle, paths)
            model = m.handle
            t.mark("update")
            t.print("check")
            return m
        }

        /** kc's analysis of the module: its model, the round of it and the facts of its code. */
        private fun analyze(): KaSession {
            val t = KcTimer()
            val pool = Executors.newFixedThreadPool(Runtime.getRuntime().availableProcessors().coerceIn(1, 4)) { r ->
                Thread(null, r, "kc-pool", 16L shl 20).apply { isDaemon = true }
            }
            try {
                // The PSI, every file's tree parsed, on the pool: kc checks
                // the sources from disk meanwhile. kc takes the Kotlin files,
                // not scripts. A script is told by its extension: KtFile.isScript
                // builds the file's stubs, which parses it a second time, and
                // makes its PSI switch from the stubs to the tree later.
                val parsed = CompletableFuture.supplyAsync({ ktFiles.filter { it.virtualFile != null && it.virtualFile.extension == "kt" } }, pool)
                    .thenCompose { files ->
                        CompletableFuture.allOf(*files.map { f -> CompletableFuture.runAsync({ KcRound.preorder(f.node) }, pool) }.toTypedArray())
                            .thenApply { files }
                    }
                val m = try {
                    checked.join()
                } catch (e: CompletionException) {
                    throw e.cause ?: e
                }
                t.mark("check")
                val errors = m.diagnostics
                var facts: List<CompletableFuture<KcFileFacts>>? = null
                if (errors.isEmpty()) {
                    facts = m.files.mapIndexed { i, f -> CompletableFuture.supplyAsync({ KcOps.facts(m.handle, i, f.nodes) }, pool) }
                } else {
                    System.err.println("kc: kc can't compile the sources (${errors.size} errors), so the type-resolution rules see no types:")
                    errors.take(10).forEach { System.err.println("kc:   $it") }
                }
                val files = parsed.join()
                t.mark("psi")
                val byPath = files.associateBy { File(it.virtualFile.path).canonicalPath }
                val joined = m.files.map { f -> f to (byPath[f.path] ?: error("kc: no PSI for ${f.path}")) }
                val round = KcRound(joined, { id -> KcOps.cls(model, id) }, { fq -> KcOps.pkg(model, fq) }, emptyList(), project, errors)
                val lint = KcLint(round, joined, facts)
                t.mark("round")
                t.print("standalone files=${files.size}")
                KcMissing.log("analyzed ${files.size} files in ${(System.nanoTime() - start) / 1_000_000} ms")
                return KcAnalysis.session(KcAnalysis(round, KcModule(project, config), KcLibraryModule(project, config), lint))
            } finally {
                // Every task is submitted by now: the pool ends with them.
                pool.shutdown()
            }
        }

        private fun close() {
            // The analysis owns the handles until it ends.
            runCatching { checked.join() }
            runCatching { analysis.join() }
            environment.provider.clear()
            if (model != 0L) KcOps.close(model)
            if (handle != 0L) KcOps.close(handle)
            environment.dispose()
        }
    }
}
