package a

import kotlin.annotation.AnnotationTarget.*

@Target(FIELD)
annotation class OnField

@Target(VALUE_PARAMETER)
annotation class OnParam

@Target(PROPERTY)
annotation class OnProp

@Target(FIELD, FUNCTION)
annotation class FieldOrFun

@Target(VALUE_PARAMETER, PROPERTY, FIELD)
annotation class Anywhere

class Targets(
    @OnField @OnParam @OnProp @FieldOrFun @Anywhere val a: Int,
    @field:Marker @param:Other("p") var b: String,
    @OnParam @Anywhere c: Long,
) {
    @OnField @OnProp @Anywhere val d = c

    @field:OnField @get:Marker @set:Marker @setparam:Marker var e = 2

    @Transient val f = 3

    @JvmField @Volatile var g = 4
}
