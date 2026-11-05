// Annotation arguments that name the nested classes of the class whose
// member or constructor the annotation is on, before other classes of the
// name, as kotlin.collections.EmptyList.
import kotlin.reflect.KClass

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.CONSTRUCTOR, AnnotationTarget.PROPERTY)
annotation class Ann(val kClass: KClass<*>)

class A @Ann(Nested::class) constructor() {
    @Ann(EmptyList::class)
    fun foo() {}

    @Ann(Nested::class)
    val p = 1

    object EmptyList
    class Nested
}

fun main() {
    println(A::class.java.getMethod("foo").getAnnotation(Ann::class.java).kClass.simpleName)
    println(A::class.java.getConstructor().getAnnotation(Ann::class.java).kClass.simpleName)
}
