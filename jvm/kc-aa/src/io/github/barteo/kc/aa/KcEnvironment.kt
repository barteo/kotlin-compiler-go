package io.github.barteo.kc.aa

import com.google.devtools.ksp.processing.KSPConfig
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.intellij.diagnostic.PluginException
import com.intellij.diagnostic.PluginProblemReporter
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.fileEditor.impl.LoadTextUtil
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.vfs.VirtualFile
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
        // The application's, which the environments of a process share, as
        // KSP's standalone backend registers it: PluginException.createByClass
        // needs one.
        val app = ApplicationManager.getApplication()
        if (app.getService(PluginProblemReporter::class.java) == null) {
            app.javaClass.getMethod("registerService", Class::class.java, Any::class.java)
                .invoke(app, PluginProblemReporter::class.java, KcPluginProblemReporter)
        }
    }

    /** How many files [psiFile] parsed again because their text changed, and how many it reused. */
    var reparsed = 0
    var reused = 0

    /**
     * The PSI of [vf]: the one parsed before if the file's text is the
     * same, or a new one. A file whose text changed gets a new view
     * provider, so its old PSI (and the old round's symbols over it) are
     * left as they were.
     */
    fun psiFile(vf: VirtualFile): PsiFile? {
        val fm = (psiManager as PsiManagerEx).fileManager
        val cached = fm.findCachedViewProvider(vf)
        if (cached != null) {
            if (cached.contents.toString() == LoadTextUtil.loadText(vf).toString()) {
                reused++
                return psiManager.findFile(vf)
            }
            fm.setViewProvider(vf, null)
            reparsed++
        }
        return psiManager.findFile(vf)
    }

    fun dispose() = Disposer.dispose(disposable)
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

/** Reports a plugin's problem as a [PluginException] of no plugin, as KSP's standalone backend does. */
private object KcPluginProblemReporter : PluginProblemReporter {
    override fun createPluginExceptionByClass(errorMessage: String, cause: Throwable?, pluginClass: Class<*>) =
        PluginException(errorMessage, cause, null)
}
