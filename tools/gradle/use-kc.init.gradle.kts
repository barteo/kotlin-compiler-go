// Gradle init script that compiles Kotlin/JVM modules with kc in place of kotlinc.
//
//   KC=/path/to/kc ./gradlew assembleDebug --init-script tools/gradle/use-kc.init.gradle.kts
//
// Each KotlinCompile task (compileKotlin, compile<Variant>Kotlin) first runs kc with the task's own
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
//
// kc compiles whole modules, so the tasks are not incremental. Compiler plugins other than Compose,
// kotlinx.serialization and Parcelize are not applied; the log names any it leaves out. Included
// builds (build-logic) keep kotlinc. The script reads the tasks through reflection, as their types
// are not on an init script's class path, so it depends on the Kotlin Gradle plugin's getters
// (checked with 2.4.21).
if (gradle.parent == null) {
  allprojects {
    tasks.matching {
      Regex("compile\\w*Kotlin").matches(it.name) &&
        it.javaClass.name.startsWith("org.jetbrains.kotlin.gradle.tasks.KotlinCompile")
    }.configureEach {
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
        val plugins = (call(task, "getPluginClasspath") as org.gradle.api.file.FileCollection).files.map { it.name }

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
        val known = mapOf(
          "compose-compiler-plugin" to "-compose",
          "serialization-compiler-plugin" to "-serialization",
          "parcelize-compiler" to "-parcelize",
        )
        val left = mutableListOf<String>()
        for (jar in plugins) {
          val flag = known.entries.firstOrNull { jar.contains(it.key) }?.value
          when {
            flag != null -> if (flag !in args) args += flag
            jar.contains("compiler-plugin") || jar.contains("-plugin-") -> left += jar
          }
        }
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
