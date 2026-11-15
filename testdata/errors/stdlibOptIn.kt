// The standard library's opt-in markers and ERROR deprecations are read
// from its class files: kotlin.io.path's ExperimentalPathApi has level
// ERROR, ExperimentalUnsignedTypes has level WARNING (not reported), and
// toUpperCase is @DeprecatedSinceKotlin(errorSince = "2.1").
@file:OptIn(kotlin.ExperimentalStdlibApi::class)

import java.nio.file.Path
import kotlin.io.path.*

fun needs(p: Path) = p.deleteRecursively() // ERROR this declaration needs opt-in: its usage must be marked with '@ExperimentalPathApi'

@OptIn(ExperimentalPathApi::class)
fun optedIn(p: Path) = p.deleteRecursively()

@Suppress("OPT_IN_USAGE_ERROR")
fun suppressed(p: Path) = p.deleteRecursively()

@ExperimentalPathApi
fun propagated(p: Path) = p.deleteRecursively()

fun caller(p: Path) = propagated(p) // ERROR needs opt-in

@OptIn(ExperimentalPathApi::class)
class Walker {
    fun walk(p: Path) = p.walk()
}

fun warningLevel(a: UIntArray) = a.size

fun deprecated(s: String) = s.toUpperCase() // ERROR 'toUpperCase' is deprecated with level ERROR
