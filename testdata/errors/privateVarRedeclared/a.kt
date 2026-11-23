package a

private val invalidProp0 = 1 // ERROR conflicting declarations: invalidProp0

fun useInvalidProp0() = invalidProp0 // ERROR overload resolution ambiguity: invalidProp0
