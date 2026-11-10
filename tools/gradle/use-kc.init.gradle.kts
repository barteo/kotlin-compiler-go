// Gradle init script that compiles Kotlin/JVM modules with kc in place of kotlinc.
//
//   KC=/path/to/kc ./gradlew assembleDebug --init-script tools/gradle/use-kc.init.gradle.kts
//
// Each KotlinCompile task (compileKotlin, compile<Variant>Kotlin, a multiplatform module's
// compileAndroidMain or compileKotlinJvm) first runs kc with the task's own
// sources, Java sources, class path, friend paths, module name and compiler plugins, writing into the
// task's output directory, and then skips kotlinc. Dexing, R8, packaging and javac consume kc's
// classes as they would kotlinc's.
//
// Environment:
//   KC         kc binary (default: kc on PATH)
//   KC_STRICT  1: a kc failure fails the build. Otherwise kotlinc compiles that module instead,
//              from scratch, and the build log says so.
//   KC_SKIP    comma-separated task paths to leave to kotlinc, such as :app:compileDebugKotlin
//   KC_JDK_HOME  JDK for modules without android.jar, in place of the task's Java toolchain
//   KC_IGNORE_PLUGINS  comma-separated compiler plugins, as artifact or group:artifact, that kc
//              may leave out because the program works without them (default:
//              compose-stability-compiler, which adds recomposition tracing and stability reports).
//              Kotlin's scripting plugin, which only compiles .kts scripts, is always left out.
//
// kc applies the Compose, kotlinx.serialization and Parcelize plugins. A module that uses any other
// compiler plugin, such as one that generates code its program needs, is left to kotlinc, with a
// warning that names the plugin, or fails the build under KC_STRICT=1. kc compiles whole modules,
// so the tasks are not incremental. Included builds (build-logic) keep kotlinc. The script reads the tasks through reflection, as their types
// are not on an init script's class path, so it depends on the Kotlin Gradle plugin's getters
// (checked with 2.4.21).
if (gradle.parent == null) {
  allprojects {
    // Kotlin/JVM compilations, Android's and a multiplatform module's JVM and Android targets'
    // included; not the multiplatform metadata (KotlinCompileCommon) or native ones.
    tasks.matching {
      it.javaClass.name.removeSuffix("_Decorated") == "org.jetbrains.kotlin.gradle.tasks.KotlinCompile"
    }.configureEach {
      // A new kc binary recompiles the module.
      System.getenv("KC")?.let { kc ->
        inputs.file(kc).withPropertyName("kcBinary").withPathSensitivity(PathSensitivity.NONE)
      }
      doFirst {
        val task = this
        fun call(target: Any, name: String): Any? =
          target.javaClass.methods.first { it.name == name && it.parameterCount == 0 }.invoke(target)
        fun callPrefixed(target: Any, prefix: String): Any? =
          target.javaClass.methods.firstOrNull { it.name.startsWith(prefix) && it.parameterCount == 0 }
            ?.invoke(target)
        fun files(name: String) =
          (call(task, name) as org.gradle.api.file.FileCollection).asFileTree.files
        fun option(name: String) =
          (call(call(task, "getCompilerOptions")!!, name) as org.gradle.api.provider.Provider<*>).orNull

        if (task.path in System.getenv("KC_SKIP").orEmpty().split(',').map { it.trim() }) {
          return@doFirst
        }
        val kc = System.getenv("KC") ?: "kc"
        val strict = System.getenv("KC_STRICT") == "1"
        val dest = (call(task, "getDestinationDirectory") as org.gradle.api.file.DirectoryProperty).get().asFile
        val sources = (files("getSources") + files("getJavaSources"))
          .filter { it.name.endsWith(".kt") || it.name.endsWith(".java") }
          .map { it.path }.distinct().sorted()
        if (sources.none { it.endsWith(".kt") }) return@doFirst
        val libraries = (call(task, "getLibraries") as org.gradle.api.file.FileCollection).files
          .filter { it.exists() }.map { it.path }
        val friends = (call(task, "getFriendPaths") as org.gradle.api.file.FileCollection).files.map { it.path }
        // The plugins themselves, not their dependencies: the jars that register one.
        val registrars = listOf("CompilerPluginRegistrar", "ComponentRegistrar")
          .map { "META-INF/services/org.jetbrains.kotlin.compiler.plugin.$it" }
        val pluginJars = (call(task, "getPluginClasspath") as org.gradle.api.file.FileCollection).files
          .filter { f ->
            f.isFile && runCatching {
              java.util.zip.ZipFile(f).use { z -> registrars.any { z.getEntry(it) != null } }
            }.getOrDefault(false)
          }
        // group:artifact, for a jar in Gradle's cache (files-2.1/group/artifact/version/hash/jar).
        fun coordinates(jar: File): String {
          val parts = jar.path.split(File.separatorChar)
          val i = parts.indexOf("files-2.1")
          return if (i >= 0 && parts.size > i + 2) "${parts[i + 1]}:${parts[i + 2]}" else jar.nameWithoutExtension
        }
        val plugins = pluginJars.map { it.name }
        val known = mapOf(
          "compose-compiler-plugin" to "-compose",
          "serialization-compiler-plugin" to "-serialization",
          "parcelize-compiler" to "-parcelize",
        )
        val ignored = (System.getenv("KC_IGNORE_PLUGINS") ?: "compose-stability-compiler")
          .split(',').map { it.trim() }.filter { it.isNotEmpty() } + "kotlin-scripting-compiler-embeddable"
        val unsupported = pluginJars.map { coordinates(it) }.filter { c ->
          known.keys.none { c.contains(it) } && ignored.none { c == it || c.substringAfter(':') == it }
        }
        if (unsupported.isNotEmpty()) {
          val why = "${task.path} uses compiler plugins kc does not apply: ${unsupported.joinToString()}"
          if (strict) throw GradleException("kc: $why (KC_IGNORE_PLUGINS names plugins to leave out)")
          logger.warn("kc: $why, so kotlinc compiles it")
          return@doFirst
        }

        val args = mutableListOf(kc, "-d", dest.path)
        (option("getModuleName") as String?)?.let { args += listOf("-module-name", it) }
        // An Android module's class path holds android.jar, which stands in for the JDK. Other
        // modules compile against the JDK of the task's Java toolchain, as kotlinc does.
        val toolchainJdk = runCatching {
          val toolchain = call(task, "getKotlinJavaToolchain")!!
          val java = callPrefixed(toolchain, "getJavaExecutable") as org.gradle.api.file.RegularFileProperty
          java.get().asFile.canonicalFile.parentFile.parentFile.path
        }.getOrNull()
        when {
          option("getNoJdk") == true || libraries.any { File(it).name == "android.jar" } -> args += "-no-jdk"
          System.getenv("KC_JDK_HOME") != null -> args += listOf("-jdk-home", System.getenv("KC_JDK_HOME"))
          toolchainJdk != null -> args += listOf("-jdk-home", toolchainJdk)
        }
        // kotlinc's default is enable: interface default methods plus DefaultImpls classes.
        val jvmDefault = option("getJvmDefault")?.let { call(it, "getCompilerArgument") as String }
          ?: (option("getFreeCompilerArgs") as List<*>?).orEmpty()
            .firstNotNullOfOrNull { (it as? String)?.removePrefix("-jvm-default=")?.takeIf { v -> v != it } }
          ?: "enable"
        args += listOf("-jvm-default", jvmDefault)
        if (libraries.isNotEmpty()) args += listOf("-cp", libraries.joinToString(File.pathSeparator))
        if (friends.isNotEmpty()) args += listOf("-friend-paths", friends.joinToString(File.pathSeparator))
        for (jar in plugins) {
          val flag = known.entries.firstOrNull { jar.contains(it.key) }?.value
          if (flag != null && flag !in args) args += flag
        }
        val left = plugins.filter { jar -> known.keys.none { jar.contains(it) } }
        if (left.isNotEmpty()) logger.info("kc: ${task.path} leaves out compiler plugins $left")
        logger.info("kc: ${task.path}: ${args.joinToString(" ")} <${sources.size} files>")
        args += sources

        dest.deleteRecursively()
        dest.mkdirs()
        val started = System.nanoTime()
        val process = ProcessBuilder(args).redirectErrorStream(true).start()
        val output = process.inputStream.bufferedReader().readText()
        val exit = process.waitFor()
        val ms = (System.nanoTime() - started) / 1_000_000
        if (exit == 0) {
          if (output.isNotBlank()) logger.info(output)
          // KGP's incremental caches describe kotlinc's last output, not kc's.
          for (prefix in listOf("getTaskBuildCacheableOutputDirectory", "getTaskBuildLocalStateDirectory")) {
            (callPrefixed(task, prefix) as? org.gradle.api.file.DirectoryProperty)?.orNull?.asFile?.deleteRecursively()
          }
          logger.lifecycle("kc: ${task.path}: ${sources.size} files in $ms ms")
          throw StopExecutionException()
        }
        val excerpt = output.lineSequence().filter { it.isNotBlank() }.take(20).joinToString("\n")
        if (strict) throw GradleException("kc failed on ${task.path}:\n$excerpt")
        logger.warn("kc failed on ${task.path}, so kotlinc compiles it:\n$excerpt")
        dest.deleteRecursively()
        task.javaClass.getMethod("setIncremental", Boolean::class.javaPrimitiveType).invoke(task, false)
      }
    }
  }
}
