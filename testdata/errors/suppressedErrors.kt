@Deprecated("gone", level = DeprecationLevel.ERROR)
fun gone() {}
@Suppress("DEPRECATION_ERROR")
fun usesGone() = gone()
fun usesGoneLoud() = gone() // ERROR is deprecated with level ERROR

class Secret { private fun hidden() = 1 }
@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun peek() = Secret().hidden()

@Suppress("VAL_REASSIGNMENT")
fun reassign() {
    val x = 1
    x = 2
}

open class Base { fun fixed() {} }
class Derived : Base() {
    @Suppress("OVERRIDING_FINAL_MEMBER")
    override fun fixed() {}
}

@Suppress("ABSTRACT_FUNCTION_IN_NON_ABSTRACT_CLASS")
class NotAbstract { abstract fun f() }

@Suppress("TYPE_MISMATCH")
val mismatched: String = 1
val mismatchedLoud: String = 1 // ERROR type mismatch

@Suppress("NOTHING_TO_OVERRIDE")
fun unrelated(s: String?) = s.length // ERROR only safe
