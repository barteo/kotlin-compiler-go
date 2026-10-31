// Annotation arguments that name constants: Java's, imported, a
// companion's from a constructor parameter's annotation, and an object's
// declared after the annotation.
import java.util.Calendar.MONDAY
import java.util.Calendar.TUESDAY
import java.time.DayOfWeek

@Retention(AnnotationRetention.RUNTIME)
annotation class Named(val value: String)

@Retention(AnnotationRetention.RUNTIME)
annotation class Mode(val mode: Int)

class Login(@Named(KEY) val prefs: String) {
    companion object {
        const val KEY = "secure"
    }
}

object Module {
    @Named(RATING)
    fun provide(): String = "x"

    internal const val RATING = "rating"
}

class Rule(@param:Named(Module.RATING) val prefs: String)

@Mode(MONDAY or TUESDAY)
class Preview

// Parameter annotations of constructors that take synthetic parameters
// first: an inner class's outer instance, an enum's name and ordinal.
class Outer {
    inner class Inner(@Named("inner") val s: String, val n: Int)
}

enum class Level(@Named("level") val label: String) { LOW("l"), HIGH("h") }

fun paramNames(c: Class<*>) = c.declaredConstructors[0].parameterAnnotations.map { ps -> ps.map { (it as Named).value } }

fun main() {
    val login = Login::class.java.constructors[0].parameterAnnotations[0][0] as Named
    println(login.value)
    println(Module::class.java.getMethod("provide").getAnnotation(Named::class.java).value)
    val rule = Rule::class.java.constructors[0].parameterAnnotations[0][0] as Named
    println(rule.value)
    println(Preview::class.java.getAnnotation(Mode::class.java).mode)
    println(paramNames(Outer.Inner::class.java))
    println(paramNames(Level::class.java))
    // A Java enum's entries.
    println(DayOfWeek.entries.joinToString("") { it.name.take(2) })
    println(DayOfWeek.entries == DayOfWeek.entries)
}
