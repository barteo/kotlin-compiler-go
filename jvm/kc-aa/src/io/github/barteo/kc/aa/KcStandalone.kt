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
    /**
     * Whether kc's check starts as early as it can (`-Dkc.prestart`, default
     * true): when the linter calls [prestart], or else before the session
     * builds its environment.
     */
    private val prestart = System.getProperty("kc.prestart") != "false"

    /** The check [prestart] started, until a session takes it. */
    private var early: Check? = null

    /**
     * Starts kc's check of [config]'s module before the session is opened,
     * as soon as a linter knows its sources, class path and options:
     * detekt-cli calls it before it reads its configuration and builds its
     * compiler configuration. [open] takes it over if it gets the same
     * module; otherwise it is closed when it ends.
     */
    @JvmStatic
    fun prestart(config: KcModuleConfig) {
        if (!prestart) return
        val c = Check(config)
        synchronized(this) {
            if (early != null) return
            early = c
        }
        c.start()
    }

    /** The check of [config]'s module: [prestart]'s if it checks the same, else a new one, not started. */
    private fun take(config: KcModuleConfig): Check {
        val key = Check.Key.of(config)
        val e = synchronized(this) { early.also { early = null } }
        if (e != null) {
            if (e.key == key) {
                KcMissing.log("took over the check started early")
                return e
            }
            KcMissing.log("the check started early checks another module: checking again")
            e.closeWhenDone()
        }
        return Check(config, key)
    }

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
        private val start = System.nanoTime()

        /**
         * kc's check of the module, which reads the sources from their
         * paths and needs no PSI: [prestart]'s, or started before anything
         * else, so that it runs while the session builds its IntelliJ
         * environment (`KotlinCoreEnvironment` and the class path's index)
         * and then the linter's own setup. `-Dkc.prestart=false` starts it
         * after the environment, as before.
         */
        private val check = take(config)

        init {
            if (prestart) check.start()
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
            if (!prestart) check.start()
            Thread(null, {
                try {
                    analysis.complete(analyze())
                } catch (e: Throwable) {
                    analysis.completeExceptionally(e)
                }
            }, "kc-analysis", 16L shl 20).apply { isDaemon = true }.start()
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
                val m = check.join()
                val model = m.handle
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
            runCatching { analysis.join() }
            environment.provider.clear()
            check.close()
            environment.dispose()
        }
    }

    /**
     * kc's check of a module's sources, from their paths, on a thread of
     * its own: the model and the facts of its code. It owns the model's and
     * the session's handles until [close].
     */
    internal class Check(private val config: KcModuleConfig, val key: Key = Key.of(config)) {
        /** What the check reads: two checks with the same key check the same. */
        data class Key(
            val paths: List<String>,
            val jdkHome: String?,
            val moduleName: String,
            val classPath: List<String>,
            val friendPaths: List<String>,
            val optIns: List<String>,
        ) {
            companion object {
                fun of(config: KcModuleConfig) = Key(
                    paths = config.sourceRoots.asSequence()
                        .flatMap { root -> root.walkTopDown().filter { it.isFile && it.extension == "kt" } }
                        .map { it.canonicalPath }
                        .distinct()
                        .sorted()
                        .toList(),
                    jdkHome = config.jdkHome?.path,
                    moduleName = config.moduleName,
                    classPath = (config.libraries + config.friends).map { it.path }.distinct(),
                    friendPaths = config.friends.map { it.path },
                    optIns = config.optIns,
                )
            }
        }

        private val result = CompletableFuture<KcModel>()
        private val started = java.util.concurrent.atomic.AtomicBoolean()
        @Volatile private var handle = 0L

        fun start() {
            if (!started.compareAndSet(false, true)) return
            // A big stack: Go's runtime runs its calls on this thread's.
            Thread(null, {
                try {
                    result.complete(run())
                } catch (e: Throwable) {
                    result.completeExceptionally(e)
                }
            }, "kc-check", 64L shl 20).apply { isDaemon = true }.start()
        }

        /** The model, once the check is done. */
        fun join(): KcModel = try {
            result.join()
        } catch (e: CompletionException) {
            throw e.cause ?: e
        }

        private fun run(): KcModel {
            val t = KcTimer()
            if (KcMissing.trace) {
                val jvmStart = ProcessHandle.current().info().startInstant().orElse(null)
                if (jvmStart != null) KcStats.time("from the JVM's start to kc's check", java.time.Duration.between(jvmStart, java.time.Instant.now()).toNanos())
            }
            handle = KcOps.open(
                KcOps.Config(
                    jdkHome = key.jdkHome,
                    noJdk = false,
                    moduleName = key.moduleName,
                    jobs = Runtime.getRuntime().availableProcessors(),
                    classPath = key.classPath,
                    friendPaths = key.friendPaths,
                    facts = true,
                    optIns = key.optIns,
                ),
            )
            t.mark("open")
            val m = KcOps.update(handle, key.paths)
            t.mark("update")
            t.print("check")
            return m
        }

        /** Closes the model and the session, once the check has ended. */
        fun close() {
            if (!started.get()) return
            val m = runCatching { result.join() }.getOrNull()
            if (m != null && m.handle != 0L) KcOps.close(m.handle)
            if (handle != 0L) KcOps.close(handle)
        }

        /** [close], on a thread of its own: for a check no session takes. */
        fun closeWhenDone() {
            Thread({ close() }, "kc-check-close").apply { isDaemon = true }.start()
        }
    }
}
