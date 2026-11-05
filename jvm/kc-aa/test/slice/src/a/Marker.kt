package a

import kotlin.annotation.AnnotationTarget.*

@Target(CLASS, FUNCTION, PROPERTY, FIELD, PROPERTY_GETTER, VALUE_PARAMETER, TYPE_PARAMETER)
annotation class Marker

annotation class Other(val s: String)
