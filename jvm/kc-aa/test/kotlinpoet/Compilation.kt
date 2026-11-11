// The part of kotlin-compile-testing's API (dev.zacsweers.kctfork) that
// kotlinpoet's interop:ksp tests use, on the KSP fork: compile() runs KSP
// on the sources with the kc backend (-Dksp.backend) and nothing else.
// kctfork itself can't run here: it brings kotlin-compiler-embeddable,
// whose org.jetbrains.kotlin classes clash with the unshaded
// kotlin-compiler 2.4.21 the fork runs on.
//
// The generated code isn't compiled, so compile()'s exit code is KSP's.
// The class path the sources see is -Dkotlinpoet.classpath.
package com.tschuchort.compiletesting

import com.google.devtools.ksp.impl.KotlinSymbolProcessing
import com.google.devtools.ksp.processing.KSPJvmConfig
import com.google.devtools.ksp.processing.KSPLogger
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.FileLocation
import com.google.devtools.ksp.symbol.KSNode
import java.io.File

class SourceFile private constructor(val name: String, val contents: String) {
    companion object {
        fun kotlin(name: String, contents: String, trimIndent: Boolean = true) =
            SourceFile(name, if (trimIndent) contents.trimIndent() else contents)
    }
}

class KspConfig {
    var incremental = false
    val symbolProcessorProviders = mutableListOf<SymbolProcessorProvider>()
    val processorOptions = mutableMapOf<String, String>()
}

class KotlinCompilation {
    enum class ExitCode { OK, INTERNAL_ERROR, COMPILATION_ERROR, SCRIPT_EXECUTION_ERROR }

    class Result(val exitCode: ExitCode, val messages: String)

    lateinit var workingDir: File
    var inheritClassPath = false
    var verbose = true
    var sources: List<SourceFile> = emptyList()
    internal val ksp = KspConfig()

    fun compile(): Result {
        val src = File(workingDir, "sources").apply { mkdirs() }
        sources.forEach { File(src, it.name).writeText(it.contents) }
        val out = File(workingDir, "ksp")
        val messages = StringBuilder()
        val logger = object : KSPLogger {
            fun log(level: String, message: String, symbol: KSNode?) {
                val at = (symbol?.location as? FileLocation)?.let { "${File(it.filePath).name}:${it.lineNumber}: " } ?: ""
                messages.append("$level: $at$message\n")
            }
            override fun logging(message: String, symbol: KSNode?) {}
            override fun info(message: String, symbol: KSNode?) {}
            override fun warn(message: String, symbol: KSNode?) = log("w", message, symbol)
            override fun error(message: String, symbol: KSNode?) = log("e", message, symbol)
            override fun exception(e: Throwable) = log("e", e.stackTraceToString(), null)
        }
        val config = KSPJvmConfig.Builder().apply {
            moduleName = "main"
            sourceRoots = listOf(src)
            javaSourceRoots = emptyList()
            commonSourceRoots = emptyList()
            libraries = System.getProperty("kotlinpoet.classpath").split(File.pathSeparator).filter { it.isNotEmpty() }.map(::File)
            jdkHome = File(System.getProperty("java.home"))
            projectBaseDir = workingDir
            outputBaseDir = out
            cachesDir = File(out, "caches")
            classOutputDir = File(out, "classes")
            kotlinOutputDir = File(out, "sources/kotlin")
            javaOutputDir = File(out, "sources/java")
            resourceOutputDir = File(out, "sources/resources")
            languageVersion = "2.4"
            apiVersion = "2.4"
            jvmTarget = "17"
            jvmDefaultMode = "enable"
            incremental = ksp.incremental
            processorOptions = ksp.processorOptions
        }.build()
        val code = try {
            KotlinSymbolProcessing(config, ksp.symbolProcessorProviders, logger).execute()
        } catch (e: Throwable) {
            logger.exception(e)
            System.err.print(messages)
            return Result(ExitCode.INTERNAL_ERROR, messages.toString())
        }
        if (messages.isNotEmpty()) System.err.print(messages)
        val exit = if (code == KotlinSymbolProcessing.ExitCode.OK) ExitCode.OK else ExitCode.COMPILATION_ERROR
        return Result(exit, messages.toString())
    }
}

fun KotlinCompilation.configureKsp(useKsp2: Boolean = true, block: KspConfig.() -> Unit) = ksp.block()

val KotlinCompilation.kspProcessorOptions: MutableMap<String, String> get() = ksp.processorOptions

val KotlinCompilation.kspSourcesDir: File get() = File(workingDir, "ksp/sources")
