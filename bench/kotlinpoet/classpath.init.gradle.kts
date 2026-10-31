// Prints the run-time class path of kotlinpoet's JVM tests, and where
// Gradle put the compiled test and library classes, for bench/kotlinpoet.sh.
gradle.projectsEvaluated {
  rootProject.findProject(":kotlinpoet")?.let { p ->
    p.tasks.register("printJvmTestClasspath") {
      val cp = p.configurations.getByName("jvmTestRuntimeClasspath")
      val tests = p.layout.buildDirectory.dir("classes/kotlin/jvm/test")
      val main = p.layout.buildDirectory.dir("classes/kotlin/jvm/main")
      val javaTests = p.layout.buildDirectory.dir("classes/java/jvmTest")
      val resources = p.layout.buildDirectory.dir("processedResources/jvm/test")
      dependsOn("jvmTestClasses")
      // The module's name, root:project, which metadata records; the
      // init script can't see the Kotlin plugin's types.
      val compile = p.tasks.getByName("compileKotlinJvm")
      val options = compile.javaClass.getMethod("getCompilerOptions").invoke(compile)
      val moduleName = options.javaClass.getMethod("getModuleName").invoke(options) as org.gradle.api.provider.Provider<*>
      doLast {
        println("KC_MODULE_NAME=" + moduleName.get())
        println("KC_TEST_CLASSES=" + tests.get().asFile)
        println("KC_JAVA_TEST_CLASSES=" + javaTests.get().asFile)
        println("KC_TEST_RESOURCES=" + resources.get().asFile)
        println("KC_MAIN_CLASSES=" + main.get().asFile)
        println("KC_RUNTIME_CLASSPATH=" + cp.files.joinToString(File.pathSeparator))
      }
    }
  }
}
