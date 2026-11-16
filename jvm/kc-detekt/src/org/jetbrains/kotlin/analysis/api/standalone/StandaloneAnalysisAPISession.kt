package org.jetbrains.kotlin.analysis.api.standalone

import com.intellij.openapi.Disposable
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiFile
import io.github.barteo.kc.detekt.KcDetekt
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.analysis.project.structure.builder.KtModuleProviderBuilder

/**
 * kc's stand-in for the Analysis API's standalone session builder, with the
 * members a client compiled against it calls: the session it builds is
 * answered by kc (io.github.barteo.kc.detekt) instead of FIR.
 */
class StandaloneAnalysisAPISessionBuilder(projectDisposable: Disposable, @Suppress("UNUSED_PARAMETER") unitTestMode: Boolean) {
    private val environment = KcDetekt.Environment(projectDisposable)

    val project: Project get() = environment.project

    private val modules = KtModuleProviderBuilder(environment.applicationEnvironment, environment.project)

    fun buildKtModuleProvider(init: KtModuleProviderBuilder.() -> Unit) {
        modules.init()
    }

    fun <T : Any> registerProjectService(serviceInterface: Class<T>, serviceImplementation: T) {
        environment.registerService(serviceInterface, serviceImplementation)
    }

    fun <T : Any> registerProjectService(serviceImplementation: Class<T>) {
        environment.registerService(serviceImplementation)
    }

    fun build(): StandaloneAnalysisAPISession {
        environment.start(modules.modules)
        return StandaloneAnalysisAPISession(project, environment.modulesWithFiles())
    }
}

class StandaloneAnalysisAPISession(val project: Project, val modulesWithFiles: Map<KaSourceModule, List<PsiFile>>)
