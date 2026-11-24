@Target(AnnotationTarget.FIELD)
annotation class Field

@Target(AnnotationTarget.PROPERTY, AnnotationTarget.FIELD)
annotation class PropertyField

@Field
val x: Int = 42

@Field // ERROR not applicable to target 'member property without backing field or delegate'
val y: Int
    get() = 13

@Field
var z: Int = 1
    get() = field
    set(v) { field = v }

abstract class My(@Field val w: Int) {
    @Field
    lateinit var l: String

    @Field // ERROR not applicable to target 'member property without backing field or delegate'
    abstract val a: Int

    @PropertyField
    abstract val b: Int

    @Field // ERROR not applicable to target 'member property without backing field or delegate'
    val v: Int by D()
}

class D {
    operator fun getValue(thisRef: Any?, p: Any?): Int = 1
}

interface His {
    @Field // ERROR not applicable to target 'member property without backing field or delegate'
    val x: Int
}

object O {
    @Field
    const val C = 1
}
