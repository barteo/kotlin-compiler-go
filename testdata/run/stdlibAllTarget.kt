// Where the annotations of a constructor's property go, as kotlinc 2.4
// places them: without a use-site target, to the parameter, and to the
// property, or else to the field, as their targets allow, so one for
// parameters and fields goes to both; with @all:, to each of the
// parameter, property, field, getter and setter's parameter that their
// targets allow.

@Target(AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.FIELD)
annotation class ParamField

@Target(AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.PROPERTY, AnnotationTarget.FIELD)
annotation class ParamPropertyField

annotation class Anywhere

@Target(AnnotationTarget.FIELD, AnnotationTarget.PROPERTY_GETTER)
annotation class FieldGetter

class C(
    @ParamField val a: Int,
    @ParamPropertyField val b: Int,
    @all:Anywhere var c: Int,
    @all:FieldGetter val d: Int,
    @all:ParamField val e: Int,
)

class M {
    @all:Anywhere var m: Int = 0
}

fun names(anns: Array<out Annotation>) = anns.map { it.annotationClass.java.simpleName }.sorted()

fun main() {
    val c = C::class.java
    for (f in c.declaredFields) {
        println("field ${f.name}: ${names(f.annotations)}")
    }
    val ctor = c.declaredConstructors.single()
    ctor.parameterAnnotations.forEachIndexed { i, a -> println("param $i: ${names(a)}") }
    for (m in c.declaredMethods.sortedBy { it.name + it.parameterCount }) {
        println("method ${m.name}: ${names(m.annotations)} params ${m.parameterAnnotations.map { names(it) }}")
    }
    for (m in M::class.java.declaredMethods.sortedBy { it.name + it.parameterCount }) {
        println("M.${m.name}: ${names(m.annotations)} params ${m.parameterAnnotations.map { names(it) }}")
    }
    println("M.m field: ${names(M::class.java.getDeclaredField("m").annotations)}")
}
