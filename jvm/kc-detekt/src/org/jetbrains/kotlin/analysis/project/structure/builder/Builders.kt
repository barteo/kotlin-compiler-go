package org.jetbrains.kotlin.analysis.project.structure.builder

import com.intellij.core.CoreApplicationEnvironment
import com.intellij.openapi.project.Project
import io.github.barteo.kc.detekt.KcLibraryModule
import io.github.barteo.kc.detekt.KcSourceModule
import org.jetbrains.kotlin.analysis.api.projectStructure.KaLibraryModule
import org.jetbrains.kotlin.analysis.api.projectStructure.KaModule
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.config.LanguageVersionSettings
import org.jetbrains.kotlin.config.LanguageVersionSettingsImpl
import org.jetbrains.kotlin.platform.TargetPlatform
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import java.nio.file.Path

// kc's stand-ins for the Analysis API's standalone module builders: they
// only record the modules, which kc analyzes as one source module with
// its dependencies' binary roots on the class path.

class KtModuleProviderBuilder(val coreApplicationEnvironment: CoreApplicationEnvironment, val project: Project) {
    var platform: TargetPlatform = JvmPlatforms.defaultJvmPlatform
    internal val modules = ArrayList<KaModule>()

    fun <M : KaModule> addModule(module: M): M {
        modules += module
        return module
    }
}

inline fun KtModuleProviderBuilder.buildKtSourceModule(init: KtSourceModuleBuilder.() -> Unit): KaSourceModule =
    KtSourceModuleBuilder(coreApplicationEnvironment, project).apply(init).build()

inline fun KtModuleProviderBuilder.buildKtLibraryModule(init: KtLibraryModuleBuilder.() -> Unit): KaLibraryModule =
    KtLibraryModuleBuilder(coreApplicationEnvironment, project, false).apply(init).build()

inline fun KtModuleProviderBuilder.buildKtSdkModule(init: KtSdkModuleBuilder.() -> Unit): KaLibraryModule =
    KtSdkModuleBuilder(coreApplicationEnvironment, project).apply(init).build()

class KtLibraryModuleBuilder(
    @Suppress("unused") val coreApplicationEnvironment: CoreApplicationEnvironment,
    val project: Project,
    private val isSdk: Boolean,
) {
    var platform: TargetPlatform = JvmPlatforms.defaultJvmPlatform
    var libraryName: String = ""
    private val roots = ArrayList<Path>()

    fun addBinaryRoot(root: Path) {
        roots.add(root)
    }

    fun addBinaryRoots(roots: Collection<Path>) {
        this.roots.addAll(roots)
    }

    fun build(): KaLibraryModule = KcLibraryModule(project, libraryName, roots.toList(), isSdk, null, platform)
}

class KtSdkModuleBuilder(@Suppress("unused") val coreApplicationEnvironment: CoreApplicationEnvironment, val project: Project) {
    var platform: TargetPlatform = JvmPlatforms.defaultJvmPlatform
    var libraryName: String = "jdk"
    private var jdkHome: Path? = null

    /** kc reads the JDK's classes from its home itself. */
    fun addBinaryRootsFromJdkHome(jdkHome: Path, @Suppress("UNUSED_PARAMETER") isJre: Boolean) {
        this.jdkHome = jdkHome
    }

    fun build(): KaLibraryModule = KcLibraryModule(project, libraryName, emptyList(), true, jdkHome, platform)
}

class KtSourceModuleBuilder(@Suppress("unused") val coreApplicationEnvironment: CoreApplicationEnvironment, val project: Project) {
    var platform: TargetPlatform = JvmPlatforms.defaultJvmPlatform
    var moduleName: String = "main"
    var languageVersionSettings: LanguageVersionSettings = LanguageVersionSettingsImpl.DEFAULT
    private val roots = ArrayList<Path>()
    private val regular = ArrayList<KaModule>()
    private val friends = ArrayList<KaModule>()

    fun addSourceRoot(root: Path) {
        roots.add(root)
    }

    fun addSourceRoots(roots: Collection<Path>) {
        this.roots.addAll(roots)
    }

    fun addRegularDependency(module: KaModule) {
        regular += module
    }

    fun addFriendDependency(module: KaModule) {
        friends += module
    }

    fun build(): KaSourceModule =
        KcSourceModule(project, moduleName, roots.toList(), regular.toList(), friends.toList(), languageVersionSettings, platform)
}
