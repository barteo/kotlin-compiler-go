// The JNI stress test (../test/Stress.java) inside a Gradle daemon: the
// "stress" task runs Stress.main in the daemon's JVM the way the KSP
// Gradle plugin runs KSP (and so kc): a worker without isolation loads it
// with a URLClassLoader kept, by class path, in a static of the plugin's
// (here the script's) classes, which the daemon keeps for later builds
// (KspAATask's isolatedClassLoaderCache). ../test.sh gradle runs it.
//
//	-Pclasses=DIR  the compiled classes of ../src and ../test
//	-Plib=FILE     libkc.so (the kc.lib property)
//	-Pargs="T N"   Stress's arguments
//	-Pextra=DIR    a directory added to the class path, which makes a
//	               new class loader
import javax.inject.Inject

interface StressParameters : WorkParameters {
    val classpath: ConfigurableFileCollection
    val lib: Property<String>
    val args: Property<String>
}

object Loaders {
    val byClasspath = HashMap<String, ClassLoader>()
}

abstract class StressWork : WorkAction<StressParameters> {
    override fun execute() {
        System.setProperty("kc.lib", parameters.lib.get())
        val urls = parameters.classpath.files.map { it.toURI().toURL() }
        val loader = synchronized(Loaders) {
            Loaders.byClasspath.getOrPut(urls.joinToString()) {
                java.net.URLClassLoader(urls.toTypedArray(), ClassLoader.getPlatformClassLoader())
            }
        }
        println("daemon pid ${ProcessHandle.current().pid()}, java ${System.getProperty("java.version")}, " +
            "LD_PRELOAD=${System.getenv("LD_PRELOAD") ?: ""}, loader ${System.identityHashCode(loader)}")
        val args = parameters.args.get().split(" ").filter { it.isNotEmpty() }.toTypedArray()
        Class.forName("Stress", true, loader).getMethod("main", Array<String>::class.java).invoke(null, args)
    }
}

abstract class Stress : DefaultTask() {
    @get:Inject abstract val workers: WorkerExecutor
    @get:InputFiles abstract val classpath: ConfigurableFileCollection
    @get:Input abstract val lib: Property<String>
    @get:Input abstract val stressArgs: Property<String>

    init {
        outputs.upToDateWhen { false }
    }

    @TaskAction
    fun run() {
        workers.noIsolation().submit(StressWork::class.java) {
            classpath.from(this@Stress.classpath)
            lib.set(this@Stress.lib)
            args.set(stressArgs)
        }
    }
}

tasks.register<Stress>("stress") {
    classpath.from(providers.gradleProperty("classes"))
    providers.gradleProperty("extra").orNull?.let { classpath.from(it) }
    lib.set(providers.gradleProperty("lib"))
    stressArgs.set(providers.gradleProperty("args").orElse(""))
}
