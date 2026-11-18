// As kotlinc, the program, in the unnamed module, can't use a JDK class
// of a package its module doesn't export to all modules
// (JAVA_MODULE_DOES_NOT_EXPORT_PACKAGE). jdk.unsupported exports sun.misc.
import sun.awt.image.*

fun withCustomDecoders(originalGetDecoder: () -> ImageDecoder?) {} // ERROR module 'java.desktop' which does not export package 'sun.awt.image'
fun createImage() = withCustomDecoders { null }
fun ok(u: sun.misc.Unsafe, l: java.util.List<String>) {}
fun internal() = jdk.internal.misc.Unsafe.getUnsafe() // ERROR module 'java.base' which does not export package 'jdk.internal.misc'
