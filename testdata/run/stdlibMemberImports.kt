import java.nio.charset.StandardCharsets.UTF_8
import java.util.Collections.nCopies
import javax.lang.model.element.NestingKind.MEMBER
import javax.lang.model.element.NestingKind.TOP_LEVEL
import javax.tools.JavaFileObject.Kind
import kotlin.DeprecationLevel.HIDDEN
import kotlin.LazyThreadSafetyMode.NONE
import kotlin.annotation.AnnotationTarget.CLASS
import kotlin.annotation.AnnotationTarget.FUNCTION
import javax.lang.model.element.NestingKind

@Target(CLASS, FUNCTION)
annotation class Marked

@Marked
class Thing

val lazyValue by lazy(NONE) { "computed" }

fun nesting(k: NestingKind) = when (k) {
    TOP_LEVEL -> "top"
    MEMBER -> "member"
    else -> "other"
}

fun main() {
    println(HIDDEN)
    println(HIDDEN.ordinal)
    println(lazyValue)
    println("é".toByteArray(UTF_8).size)
    println(Kind.SOURCE)
    println(nCopies(3, "ab"))
    println(nesting(MEMBER) + " " + nesting(TOP_LEVEL))
    val m = mapOf(1 to "one", 2 to "two")
    m.forEach { (k, v) -> println("$k=$v") }
    println(m.entries.map { (k, _) -> k * 10 })
    println(m.map { (k, v) -> v.repeat(k) })
}
