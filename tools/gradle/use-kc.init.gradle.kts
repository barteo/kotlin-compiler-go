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
//   KC_VARIANTS  comma-separated Android build types or variants, such as debug or freeDebug, to
//              compile with kc, their unit and instrumented tests included. Every other variant
//              keeps kotlinc, and its compile task is left as it is, so it never reuses classes kc
//              wrote. A module without variants, such as a Kotlin/JVM library, serves all of
//              them: kc compiles it only in a build that runs tasks of listed variants and of no
//              other. A task whose variant the script cannot tell keeps kotlinc.
//   KC_KSP     directory of kc's KSP backend (jvm/kc-aa/build.sh writes it as
//              jvm/kc-aa/build/kc-ksp). With it, the KSP tasks (kspKotlin, ksp<Variant>Kotlin) of
//              the modules and variants kc compiles run KSP on kc: their KSP class path becomes
//              the directory's loader and KSP API jars and, from the task's own, KSP's
//              configuration classes, the Kotlin standard library and kotlinx.coroutines. The
//              processors see those jars only. Unless KC_STRICT=1, a task whose run on kc fails
//              runs stock KSP in its place, and the build log says so. A daemon's builds must
//              name one directory with one libkc.so: after an upgrade of kc, restart the daemon.
//              Checked with the KSP Gradle plugin 2.3.12 and 2.3.13.
//   KC_KSP_KEEP  1: KSP on kc keeps the processors' classes loaded between the tasks and builds
//              of a daemon, for as long as the processors' jars stay the same. KSP's Gradle
//              plugin loads them anew for every task, so every run starts on code the JVM has
//              not compiled yet. A processor then keeps its static state between runs.
//   KC_KSP_FORGET  1: KSP on kc keeps nothing of a module in memory for the daemon's next build.
//              Without it, the modules whose KSP tasks ran last keep their parsed files, models
//              and class paths, so that a build after an edit reads only what changed: gigabytes
//              in a large project, for a second or less of such a build.
//   KC_JDK_HOME  JDK to compile against, in place of the task's Java toolchain
//   KC_IGNORE_PLUGINS  comma-separated compiler plugins, as artifact or group:artifact, that kc
//              may leave out because the program works without them (default:
//              compose-stability-compiler, which adds recomposition tracing and stability reports).
//              Kotlin's scripting plugin, which only compiles .kts scripts, is always left out.
//
// kc applies the Compose, kotlinx.serialization, Parcelize and Metro plugins, and passes kc the
// Metro plugin's options. A module that uses any other
// compiler plugin, such as one that generates code its program needs, is left to kotlinc, with a
// warning that names the plugin, or fails the build under KC_STRICT=1. kc compiles whole modules,
// so the tasks are not incremental. Included builds (build-logic) keep kotlinc. The script reads the tasks through reflection, as their types
// are not on an init script's class path, so it depends on the Kotlin Gradle plugin's getters
// (checked with 2.4.21).
if (gradle.parent == null) {
  // Kotlin/JVM compilations, Android's and a multiplatform module's JVM and Android targets'
  // included; not the multiplatform metadata (KotlinCompileCommon) or native ones.
  fun isKotlinCompile(task: Task) =
    task.javaClass.name.removeSuffix("_Decorated") == "org.jetbrains.kotlin.gradle.tasks.KotlinCompile"
  val useKc: Task.() -> Unit = {
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
      val plugins = pluginJars.map { coordinates(it) }
      val known = mapOf(
        "compose-compiler-plugin" to "-compose",
        "serialization-compiler-plugin" to "-serialization",
        "parcelize-compiler" to "-parcelize",
        "dev.zacsweers.metro:compiler" to "-metro",
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
      // A module compiles against its class path and then the JDK of the task's Java toolchain,
      // as kotlinc does: an Android module's android.jar stands in for the JDK's classes, and
      // its unit tests still see the ones android.jar lacks, such as java.lang.management.
      val toolchainJdk = runCatching {
        val toolchain = call(task, "getKotlinJavaToolchain")!!
        val java = callPrefixed(toolchain, "getJavaExecutable") as org.gradle.api.file.RegularFileProperty
        java.get().asFile.canonicalFile.parentFile.parentFile.path
      }.getOrNull()
      when {
        option("getNoJdk") == true -> args += "-no-jdk"
        System.getenv("KC_JDK_HOME") != null -> args += listOf("-jdk-home", System.getenv("KC_JDK_HOME"))
        toolchainJdk != null -> args += listOf("-jdk-home", toolchainJdk)
      }
      // kotlinc's default is enable: interface default methods plus DefaultImpls classes.
      val jvmDefault = option("getJvmDefault")?.let { call(it, "getCompilerArgument") as String }
        ?: (option("getFreeCompilerArgs") as List<*>?).orEmpty()
          .firstNotNullOfOrNull { (it as? String)?.removePrefix("-jvm-default=")?.takeIf { v -> v != it } }
        ?: "enable"
      args += listOf("-jvm-default", jvmDefault)
      // Explicit API mode, as kotlinc's -Xexplicit-api: kotlin { explicitApi() } sets the task's
      // explicitApiMode (an internal property, whose getter's name is mangled), and a module's
      // free compiler arguments may give it, or -XXexplicit-return-types, too.
      val freeArgs = (option("getFreeCompilerArgs") as List<*>?).orEmpty().filterIsInstance<String>()
      val explicitApi = freeArgs.lastOrNull { it.startsWith("-Xexplicit-api=") }?.substringAfter('=')
        ?: runCatching {
          val mode = (callPrefixed(task, "getExplicitApiMode") as org.gradle.api.provider.Provider<*>?)?.orNull
          when (mode?.toString()?.lowercase()) {
            "strict" -> "strict"
            "warning" -> "warning"
            else -> null
          }
        }.getOrNull()
      if (explicitApi != null) args += "-Xexplicit-api=$explicitApi"
      freeArgs.lastOrNull { it.startsWith("-XXexplicit-return-types=") }?.let { args += it }
      // A multiplatform module's JVM compilation has expect and actual declarations, which are
      // errors without -Xmulti-platform, as with kotlinc: the task's multiPlatformEnabled (an
      // internal property, whose getter's name is mangled) says so, or its free arguments do.
      val multiPlatform = "-Xmulti-platform" in freeArgs || runCatching {
        (callPrefixed(task, "getMultiPlatformEnabled") as org.gradle.api.provider.Provider<*>?)?.orNull == true
      }.getOrDefault(false)
      if (multiPlatform) args += "-Xmulti-platform"
      // The JVM version the class files are for, the module's jvmTarget, as kotlinc's -jvm-target:
      // a library's inline functions built for a higher one are errors, as with kotlinc.
      option("getJvmTarget")?.let { call(it, "getTarget") as String }?.let { args += listOf("-jvm-target", it) }
      if (libraries.isNotEmpty()) args += listOf("-cp", libraries.joinToString(File.pathSeparator))
      if (friends.isNotEmpty()) args += listOf("-friend-paths", friends.joinToString(File.pathSeparator))
      for (jar in plugins) {
        val flag = known.entries.firstOrNull { jar.contains(it.key) }?.value
        if (flag != null && flag !in args) args += flag
      }
      // Metro's options, as the Metro Gradle plugin passes them to kotlinc.
      if ("-metro" in args) {
        val configs = (call(task, "getPluginOptions") as org.gradle.api.provider.Provider<*>).get() as List<*>
        for (config in configs) {
          val all = call(config!!, "allOptions") as Map<*, *>
          for (option in (all["dev.zacsweers.metro.compiler"] as List<*>?).orEmpty()) {
            args += listOf("-P", "plugin:dev.zacsweers.metro.compiler:${call(option!!, "getKey")}=${call(option, "getValue")}")
          }
        }
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
      // The errors, which follow the warnings of the files before them.
      val lines = output.lines().filter { it.isNotBlank() }
      val excerpt = lines.filterNot { ": warning: " in it }.ifEmpty { lines }.take(20).joinToString("\n")
      if (strict) throw GradleException("kc failed on ${task.path}:\n$excerpt")
      logger.warn("kc failed on ${task.path}, so kotlinc compiles it:\n$excerpt")
      dest.deleteRecursively()
      task.javaClass.getMethod("setIncremental", Boolean::class.javaPrimitiveType).invoke(task, false)
    }
  }
  fun isKsp(task: Task) =
    task.javaClass.name.removeSuffix("_Decorated") == "com.google.devtools.ksp.gradle.KspAATask"
  val useKcKsp: Task.() -> Unit = {
    val home = System.getenv("KC_KSP")
    if (home != null && path !in System.getenv("KC_SKIP").orEmpty().split(',').map { it.trim() }) {
      val classpath = javaClass.getMethod("getKspClasspath").invoke(this) as ConfigurableFileCollection
      val stock = project.files(classpath.from.toList())
      // The directory's API is the one its KSP is compiled against. The configuration classes
      // stay the plugin's version, as the plugin passes KSP a serialized configuration.
      classpath.setFrom(
        File(home, "kc-ksp-loader.jar"),
        File(home, "symbol-processing-api.jar"),
        stock.filter { f -> listOf("api", "aa-embeddable").none { f.name.startsWith("symbol-processing-$it-") } },
      )
      inputs.file(File(home, "libkc.so")).withPropertyName("kcKspLibrary").withPathSensitivity(PathSensitivity.NONE)
      inputs.dir(File(home, "lib")).withPropertyName("kcKspBackend").withPathSensitivity(PathSensitivity.RELATIVE)
      doFirst {
        // The loader reads the property, named by its own class path, when its run fails.
        val key = "kc.ksp.stock:" + classpath.files.joinToString(File.pathSeparator) { it.path }
        if (System.getenv("KC_STRICT") == "1") {
          System.clearProperty(key)
        } else {
          System.setProperty(key, stock.files.joinToString(File.pathSeparator) { it.path })
        }
      }
    }
  }
  val listed = System.getenv("KC_VARIANTS")?.split(',')?.map { it.trim() }
  if (listed == null) {
    allprojects { tasks.matching { isKotlinCompile(it) }.configureEach { useKc(this) } }
    // Once the plugin has set the tasks' class paths.
    gradle.taskGraph.whenReady {
      for (task in allTasks) if (isKsp(task) && task.project.gradle == gradle) useKcKsp(task)
    }
  } else {
    fun call(target: Any, name: String): Any? =
      target.javaClass.methods.first { it.name == name && it.parameterCount == 0 }.invoke(target)
    // The components of each project that has an Android plugin: a variant and its test
    // components by name, each with the variant's name and build type.
    val components = mutableMapOf<Project, MutableMap<String, Pair<String, String?>>>()
    allprojects {
      val known = mutableMapOf<String, Pair<String, String?>>()
      val project = this
      pluginManager.withPlugin("com.android.base") { components[project] = known }
      val plugins = listOf("application", "library", "dynamic-feature", "test").map { "com.android.$it" }
      for (plugin in plugins) pluginManager.withPlugin(plugin) {
        runCatching {
          val extension = extensions.getByName("androidComponents")
          extension.javaClass.methods
            .first { it.name == "onVariants" && it.parameterTypes.lastOrNull() == Action::class.java }
            .invoke(extension, call(call(extension, "selector")!!, "all"), Action<Any> {
              runCatching {
                val variant = (call(this, "getName") as String) to (call(this, "getBuildType") as String?)
                for (component in call(this, "getComponents") as List<*>) {
                  known[call(component!!, "getName") as String] = variant
                }
              }
            })
        }
      }
    }
    // Tasks get kc once the build knows which ones it runs: a task that keeps kotlinc is left as
    // it is, so it stays up to date after a kotlinc build and reruns after a kc one.
    gradle.taskGraph.whenReady {
      fun isListed(variant: Pair<String, String?>) = variant.first in listed || variant.second in listed
      val running = allTasks.flatMap { task ->
        components[task.project].orEmpty().values.distinct()
          .filter { task.name.contains(it.first.replaceFirstChar { c -> c.uppercase() }) }
      }
      val onlyListed = running.isNotEmpty() && running.all { isListed(it) }
      for (task in allTasks) {
        val compile = isKotlinCompile(task)
        if (!compile && !isKsp(task) || task.project.gradle != gradle) continue
        val prefix = if (compile) "compile" else "ksp"
        val known = components[task.project]
        val variant = known?.entries?.firstOrNull {
          val name = it.key.replaceFirstChar { c -> c.uppercase() }
          task.name == "$prefix${name}Kotlin" || task.name == "$prefix${name}KotlinAndroid"
        }?.value
        // A module without variants serves every variant, so kc compiles it only in a build of
        // listed variants alone.
        if (if (known == null) onlyListed else variant != null && isListed(variant)) {
          if (compile) useKc(task) else useKcKsp(task)
        } else {
          task.logger.info("kc: KC_VARIANTS leaves ${task.path} to ${if (compile) "kotlinc" else "stock KSP"}")
        }
      }
    }
  }
}
