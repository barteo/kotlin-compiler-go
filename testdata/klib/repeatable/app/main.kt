import tags.Tag

@Tag("a")
@Tag("b")
class Twice

@Tag("one")
class Once

fun main() {
    println(Twice::class.java.getAnnotationsByType(Tag::class.java).map { it.name })
    println(Once::class.java.getAnnotationsByType(Tag::class.java).map { it.name })
    println(Twice::class.java.annotations.map { it.annotationClass.java.simpleName }.filter { it != "Metadata" })
}
