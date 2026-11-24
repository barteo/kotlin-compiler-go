// To kotlinc 2.4 (KT-66392), a Java method overriding an accessor of a
// Kotlin var with a value isn't that var's accessor: a Kotlin class
// inheriting both clashes, and one overriding the var overrides the Java
// method by accident. A val's or an abstract var's accessors are
// overridden in Java.

open class KVar {
    open var e = "1"
}
open class KVal {
    open val e: String = "1"
}
abstract class KAbstract {
    abstract var e: String
}

class Inherits : JVar() // ERROR inherited platform declarations clash
class Overrides : JVar() {
    override var e = super.e // ERROR accidental override
}

class InheritsVal : JVal()
class OverridesVal : JVal() {
    override val e = super.e
}
class InheritsAbstract : JAbstract()
