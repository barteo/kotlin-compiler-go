package io.github.barteo.kc.aa

import com.google.devtools.ksp.processing.KSPConfig
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.intellij.openapi.Disposable
import com.intellij.openapi.fileEditor.impl.LoadTextUtil
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.vfs.StandardFileSystems
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.openapi.vfs.VirtualFileManager
import com.intellij.psi.PsiFile
import com.intellij.psi.PsiManager
import com.intellij.psi.impl.PsiManagerEx
import org.jetbrains.kotlin.analysis.api.platform.modification.KotlinModificationTrackerFactory
import org.jetbrains.kotlin.analysis.api.session.KaSessionProvider
import org.jetbrains.kotlin.cli.extensionsStorage
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment
import org.jetbrains.kotlin.cli.jvm.config.addJavaSourceRoots
import org.jetbrains.kotlin.cli.jvm.config.addJvmClasspathRoots
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.config.JVMConfigurationKeys
import java.io.File

/**
 * An IntelliJ environment for KSP runs of one configuration, kept across
 * the runs of a process (a Gradle daemon): its PSI of the files the runs
 * read stays parsed, and [psiFile] parses a file again only when its text
 * has changed. It owns its disposable, not the run's. One session uses it
 * at a time ([KcEnvironments.acquire]).
 */
internal class KcEnvironment(val key: String, config: KSPConfig) {
    private val disposable: Disposable = Disposer.newDisposable("kc environment")

    val env: KotlinCoreEnvironment = run {
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

    val project get() = env.project
    val provider = KcSessionProvider(project)
    private val psiManager = PsiManager.getInstance(project)

    init {
        // MockProject.registerService, called reflectively: kotlinc can't see
        // MockProject's supertypes in kotlin-compiler's IntelliJ core.
        val register = project.javaClass.getMethod("registerService", Class::class.java, Any::class.java)
        register.invoke(project, KaSessionProvider::class.java, provider)
        register.invoke(project, KotlinModificationTrackerFactory::class.java, KcModificationTrackers)
    }

    /** How many files [psiFile] parsed again because their text changed, and how many it reused. */
    var reparsed = 0
    var reused = 0

    /**
     * The PSI of the file at [path], whose text libkc read has hash [text]
     * ([KcFile.text]): the PSI [psiFile] gave for it before, without
     * reading the file again, if the bytes it read then have that hash and
     * the PSI is still the file's; otherwise [psiFile]'s.
     */
    fun psiFile(path: String, text: Long): PsiFile? {
        parsed[path]?.let { p ->
            if (text != 0L && p.hash == text && (psiManager as PsiManagerEx).fileManager.findCachedViewProvider(p.vf) != null) {
                reused++
                return psiManager.findFile(p.vf)
            }
        }
        return files.refreshAndFindFileByPath(path)?.let { psiFile(it, path) }
    }

    /**
     * The PSI of [vf]: the one parsed before if the file's text is the
     * same, or a new one. A file whose text changed gets a new view
     * provider, so its old PSI (and the old round's symbols over it) are
     * left as they were.
     */
    fun psiFile(vf: VirtualFile, path: String = vf.path): PsiFile? {
        val fm = (psiManager as PsiManagerEx).fileManager
        val cached = fm.findCachedViewProvider(vf)
        // The file's bytes, compared with those its PSI was parsed from:
        // cheaper than decoding its text again.
        val bytes = vf.contentsToByteArray()
        parsed[path] = Parsed(vf, textHash(bytes))
        if (cached != null) {
            val same = parsedBytes[vf]?.contentEquals(bytes)
                ?: (cached.contents.toString() == LoadTextUtil.loadText(vf).toString())
            if (same) {
                reused++
                parsedBytes[vf] = bytes
                return psiManager.findFile(vf)
            }
            fm.setViewProvider(vf, null)
            reparsed++
        }
        parsedBytes[vf] = bytes
        return psiManager.findFile(vf)
    }

    private val files = VirtualFileManager.getInstance().getFileSystem(StandardFileSystems.FILE_PROTOCOL)

    /**
     * Has the PSI of the files at [paths] made and parsed on another
     * thread, while the caller waits for libkc's update: the files whose
     * PSI the round after the update would otherwise make and parse
     * itself (KcSession.update). A file is read again here, and [psiFile]
     * given libkc's hash of the same bytes takes the PSI made here. A
     * Kotlin file's tree waits in [trees] for the round's join; a Java
     * file's classes are loaded, which parses it. The PSI is made one
     * file after another ([psiFile] isn't thread-safe), and parsed in
     * parallel when there are many files. The caller touches nothing of
     * this environment until the result is done, and a file that fails is
     * left to the round.
     */
    fun parseAhead(paths: List<String>): java.util.concurrent.CompletableFuture<Void> {
        val submitted = System.nanoTime()
        return java.util.concurrent.CompletableFuture.runAsync {
            val t = KcTimer(submitted)
            t.mark("start")
            val made = paths.mapNotNull { p ->
                try {
                    files.refreshAndFindFileByPath(p)?.let { psiFile(it, p) }
                } catch (e: Exception) {
                    null // The round makes the file's PSI itself.
                }
            }
            t.mark("psi")
            val parse = { f: PsiFile ->
                try {
                    when (f) {
                        is org.jetbrains.kotlin.psi.KtFile -> trees.ahead(f, KcRound.preorder(f.node))
                        is com.intellij.psi.PsiJavaFile -> {
                            f.packageName
                            fun walk(c: com.intellij.psi.PsiClass) { c.innerClasses.forEach(::walk) }
                            f.classes.forEach(::walk)
                        }
                    }
                } catch (e: Exception) {
                    // The round parses the file itself.
                }
            }
            if (made.size > SMALL_AHEAD) made.parallelStream().forEach(parse) else made.forEach(parse)
            t.mark("parse")
            val kt = made.count { it is org.jetbrains.kotlin.psi.KtFile }
            t.print("ahead kt=$kt java=${made.size - kt}")
        }
    }

    /** Whether [psiFile] made the PSI of the file at [path] before. */
    fun loaded(path: String): Boolean = path in parsed

    /** A file's PSI's virtual file, and the hash of the bytes [psiFile] last read from it. */
    private class Parsed(val vf: VirtualFile, val hash: Long)

    /** By path, as [psiFile] was asked for it. */
    private val parsed = HashMap<String, Parsed>()

    /** The trees of the files' PSI that rounds joined, for later rounds over the same PSI. */
    val trees = KcTrees()

    /**
     * The class-path classes libkc modeled, by outermost class ID, for the
     * sessions after (see KcSession.libraryDecl): the environment's key
     * holds the class path's files and stamps, and [libraryDeclsOf]
     * starts over for another module name or other friends.
     */
    private val libraryDecls = LibraryDecls()
    private var libraryDeclsKey: String? = null

    fun libraryDeclsOf(key: String): LibraryDecls = synchronized(libraryDecls) {
        if (libraryDeclsKey != key) {
            libraryDecls.clear()
            libraryDeclsKey = key
        }
        libraryDecls
    }

    /**
     * What libkc said of the classes of a class path: the classes it
     * modeled, the names it has no class of, and the classes of the last
     * [MAX_JAVA] sets of Java sources it modeled over it, by their
     * models' KcModel.java (null for a class it didn't model), most
     * recently used last. A KSP run's rounds and the runs after see the
     * same few sets: a round's generated Java sources are the next
     * round's, again and again.
     */
    class LibraryDecls {
        val decls = HashMap<String, KcDecl>()
        val absent = HashSet<String>()
        private val java = LinkedHashMap<Long, HashMap<String, KcDecl?>>(8, 0.75f, true)

        /** The classes of the Java sources of models whose KcModel.java is [id]. */
        fun java(id: Long): HashMap<String, KcDecl?> = synchronized(this) {
            java.getOrPut(id) { HashMap() }.also {
                while (java.size > MAX_JAVA) java.remove(java.keys.first())
            }
        }

        fun clear() = synchronized(this) {
            decls.clear()
            absent.clear()
            java.clear()
        }
    }

    /** The bytes each file's PSI was parsed from, as [psiFile] last read them. */
    private val parsedBytes = HashMap<VirtualFile, ByteArray>()

    /** A round kept for a later update of the same check, with its model's handle, which the environment owns. */
    class KeptRound(
        val round: KcRound,
        val handle: Long,
        val diagnostics: List<KcDiagnostic>,
        val reparsed: Int,
        /** The model's files by path, which a later model may share (KcOps.update's bases). */
        val files: Map<String, KcFile>,
    )

    /**
     * The last rounds built in this environment, by their models' checks
     * (Go's Model.Check), least recently used first: an update that gets
     * one of these checks back uses its round again, if [psiFile] has
     * parsed no file again since, so that its PSI is the files' PSI.
     */
    private val rounds = LinkedHashMap<Long, KeptRound>()

    /** The checks of the rounds an update may use again. */
    fun reusableChecks(): List<Long> = rounds.filterValues { it.reparsed == reparsed }.keys.toList()

    /** The files of the kept rounds' models, by their checks: those an update's model may share. */
    fun baseFiles(): Map<Long, Map<String, KcFile>> = rounds.mapValues { it.value.files }

    /** The round kept for [check], now the most recently used. */
    fun reuse(check: Long): KeptRound {
        val k = rounds.remove(check) ?: error("kc: no round kept for check $check")
        rounds[check] = k
        return k
    }

    /** Keeps [round] for later updates of [check], dropping the least recently used beyond [KEEP_ROUNDS]. */
    fun keep(check: Long, round: KcRound, handle: Long, diagnostics: List<KcDiagnostic>, files: List<KcFile>) {
        rounds.remove(check)?.let(::drop)
        rounds[check] = KeptRound(round, handle, diagnostics, reparsed, files.associateBy { it.path })
        while (rounds.size > KEEP_ROUNDS) {
            val first = rounds.keys.first()
            drop(rounds.remove(first)!!)
        }
    }

    private fun drop(k: KeptRound) {
        k.round.token.invalidation = "dropped from the kept rounds"
        KcOps.close(k.handle)
    }

    fun dispose() {
        rounds.values.forEach(::drop)
        rounds.clear()
        Disposer.dispose(disposable)
    }

    private companion object {
        /** A KSP run's rounds that check different sources: the first, and the ones with generated files. */
        const val KEEP_ROUNDS = 3

        /** The sets of Java sources LibraryDecls keeps the classes of: a round's, and a few more. */
        const val MAX_JAVA = KEEP_ROUNDS + 1

        /** Up to this many files, [parseAhead] parses them one after another. */
        const val SMALL_AHEAD = 8
    }
}

/**
 * The environments of a process, by configuration: the class path (each
 * jar's size and modification time too), the JDK, the Java source roots
 * and the Java sources in them. Java PSI resolves against the Java file
 * manager's index of the roots, which is built once, so a change to the
 * Java sources makes a new environment; a Kotlin source's change only
 * reparses that file. The two environments no session uses that were used
 * last are kept.
 */
internal object KcEnvironments {
    private const val KEEP = 2
    private val idle = ArrayList<KcEnvironment>() // least recently used first
    private val enabled = System.getProperty("kc.keepEnvironment") != "false"

    @Synchronized
    fun acquire(config: KSPConfig, javaSources: List<String>): KcEnvironment {
        val key = key(config, javaSources)
        if (enabled) {
            val i = idle.indexOfLast { it.key == key }
            if (i >= 0) return idle.removeAt(i)
        }
        return KcEnvironment(key, config)
    }

    @Synchronized
    fun release(e: KcEnvironment) {
        e.provider.clear()
        if (!enabled) {
            e.dispose()
            return
        }
        idle += e
        while (idle.size > KEEP) idle.removeAt(0).dispose()
    }

    private fun key(config: KSPConfig, javaSources: List<String>): String {
        val jvm = config as? KSPJvmConfig
        fun stamp(f: File) = "${f.path}:${f.length()}:${f.lastModified()}"
        return buildString {
            (config.libraries + config.friends).distinct().forEach { append(stamp(it)).append('\n') }
            jvm?.jdkHome?.let { append(stamp(File(it, "lib/modules"))).append('\n') }
            jvm?.javaSourceRoots.orEmpty().forEach { append("root=").append(it.path).append('\n') }
            javaSources.forEach { append(stamp(File(it))).append('\n') }
        }
    }
}

/** The 64-bit FNV-1a hash of [bytes]: libkc's hash of a file's text (kaops's textHash). */
internal fun textHash(bytes: ByteArray): Long {
    var h = -0x340d631b7bdddcdbL // 14695981039346656037
    for (b in bytes) {
        h = h xor (b.toLong() and 0xff)
        h *= 0x100000001b3L
    }
    return h
}
