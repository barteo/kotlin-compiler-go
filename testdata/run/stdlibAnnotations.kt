// Annotations that class files keep: read back with Java reflection,
// with their arguments, defaults and targets.

@Retention(AnnotationRetention.RUNTIME)
annotation class Info(
    val name: String = "none",
    val level: Int = 1,
    val kind: DeprecationLevel = DeprecationLevel.WARNING,
    val type: kotlin.reflect.KClass<*> = Any::class,
    val tags: Array<String> = [],
)

@Retention(AnnotationRetention.BINARY)
annotation class Invisible

annotation class Marker

@Info(name = "box", level = 2, kind = DeprecationLevel.ERROR, type = String::class, tags = ["a", "b"])
@Invisible
class Box(@param:Marker val size: Int) {
    @Info("prop") var label: String = ""
    @get:Marker val doubled: Int get() = size * 2
    @field:Info("field") val stored: Int = 3

    @Deprecated("use other", ReplaceWith("other()"))
    @Info(tags = ["x"])
    fun old(@Marker p: String): String = p
}

fun describe(a: Annotation): String = when (a) {
    is Info -> "Info(" + a.name + ", " + a.level + ", " + a.kind + ", " + a.tags.joinToString("|") + ")"
    is Marker -> "Marker"
    is Deprecated -> "Deprecated(" + a.message + ", " + a.replaceWith.expression + ", " + a.level + ")"
    else -> "other"
}

fun names(anns: Array<Annotation>): String = anns.filter { it !is Metadata }.map { describe(it) }.sorted().joinToString(", ")

fun main() {
    val c = Box::class.java
    println("class: " + names(c.annotations))
    val old = c.getMethod("old", String::class.java)
    println("old: " + names(old.annotations))
    println("old param: " + names(old.parameterAnnotations[0]))
    println("getDoubled: " + names(c.getMethod("getDoubled").annotations))
    println("stored field: " + names(c.getDeclaredField("stored").annotations))
    println("label property: " + names(c.getMethod("getLabel\$annotations").annotations))
    println("ctor param: " + names(c.getConstructor(Int::class.javaPrimitiveType).parameterAnnotations[0]))
    println("deprecated: " + old.isAnnotationPresent(java.lang.Deprecated::class.java))
    println("default: " + Info::class.java.getMethod("level").defaultValue)
}
