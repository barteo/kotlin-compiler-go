// Makes kc compile kotlinpoet's JVM library in kotlinpoet's own build,
// for bench/kotlinpoet.sh: the action of :kotlinpoet:compileKotlinJvm
// runs kc, the path in the KC environment variable, on the task's
// sources, against its libraries, into its output directory, with its
// module name. Everything that depends on the library (its jar, its tests,
// the interop modules) then uses kc's classes, compiled against by
// kotlinc, and the tests' friend paths see its internal declarations.
//
// The init script can't see the Kotlin plugin's types, so it calls the
// task's getters by reflection.
gradle.projectsEvaluated {
  val kc = System.getenv("KC") ?: error("set KC to the kc binary")
  rootProject.findProject(":kotlinpoet")?.tasks?.named("compileKotlinJvm")?.configure {
    val task = this
    fun get(target: Any, name: String): Any = target.javaClass.getMethod(name).invoke(target)
    val dest = get(task, "getDestinationDirectory") as org.gradle.api.file.DirectoryProperty
    val sources = get(task, "getSources") as org.gradle.api.file.FileCollection
    val libraries = get(task, "getLibraries") as org.gradle.api.file.FileCollection
    val options = get(task, "getCompilerOptions")
    val moduleName = get(options, "getModuleName") as org.gradle.api.provider.Provider<*>
    actions.clear()
    doLast {
      val out = dest.get().asFile
      out.deleteRecursively()
      out.mkdirs()
      val cp = libraries.files.filter { it.exists() }.joinToString(File.pathSeparator)
      val cmd = listOf(kc, "-module-name", moduleName.get().toString(), "-cp", cp, "-d", out.path) +
        sources.files.filter { it.name.endsWith(".kt") }.map { it.path }
      val proc = ProcessBuilder(cmd).redirectErrorStream(true).start()
      val output = proc.inputStream.bufferedReader().readText()
      if (proc.waitFor() != 0) {
        throw GradleException("kc failed:\n$output")
      }
      logger.lifecycle("kc compiled ${sources.files.size} files into $out")
    }
  }
}
