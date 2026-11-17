package io.github.barteo.kc.aa

import com.intellij.openapi.project.Project
import org.jetbrains.kotlin.analysis.api.projectStructure.KaModule
import org.jetbrains.kotlin.config.JvmTarget
import org.jetbrains.kotlin.config.LanguageVersionSettings
import org.jetbrains.kotlin.config.LanguageVersionSettingsImpl
import org.jetbrains.kotlin.platform.TargetPlatform
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import java.io.File

/**
 * The module kc analyzes, whoever asks: KSP's configuration
 * ([KcBackend]) or a linter's ([KcStandalone]). Nothing here is KSP's, so
 * that a client without KSP on its class path can use kc-aa.
 */
class KcModuleConfig(
    val moduleName: String,
    /** The roots of the Kotlin sources. */
    val sourceRoots: List<File>,
    /** The roots of the Java sources; a module without any isn't a JVM one ([jvm]). */
    val javaSourceRoots: List<File> = emptyList(),
    val libraries: List<File> = emptyList(),
    /** The libraries whose internal declarations the module sees. */
    val friends: List<File> = emptyList(),
    val jdkHome: File? = null,
    val jvmTarget: String? = null,
    /** The module's -opt-in markers. */
    val optIns: List<String> = emptyList(),
    val languageVersionSettings: LanguageVersionSettings = LanguageVersionSettingsImpl.DEFAULT,
    /** Whether the module is a JVM one, whose Java sources and class path the IntelliJ environment reads. */
    val jvm: Boolean = true,
) {
    /**
     * The module's source roots: Kotlin and Java sources are read from each
     * of them, whichever list names it, as stock KSP's source module does.
     */
    val allSourceRoots: List<File> get() = (sourceRoots + javaSourceRoots).distinct()
}

/** The module kc analyzes. */
internal class KcModule(override val project: Project, private val config: KcModuleConfig) : StubKaSourceModule() {
    override val name: String get() = config.moduleName
    override val targetPlatform: TargetPlatform
        get() = config.jvmTarget?.let(JvmTarget::fromString)
            ?.let(JvmPlatforms::jvmPlatformByTargetVersion) ?: JvmPlatforms.defaultJvmPlatform
    override val languageVersionSettings: LanguageVersionSettings get() = config.languageVersionSettings
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
internal class KcLibraryModule(override val project: Project, private val config: KcModuleConfig) : StubKaLibraryModule() {
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
