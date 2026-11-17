// super.fn() is ambiguous between a function of one supertype and a
// property of another, invokable or not, but not a Java getter, nor an
// abstract property where the other supertype implements fn.
interface I { fun fn(): String = "" }

open class Lambda { val fn = { "x" } }
class C1 : Lambda(), I { fun foo() = super.fn() } // ERROR many supertypes available

open class NotInvokable { val fn = 1 }
class C2 : NotInvokable(), I { fun foo() = super.fn() } // ERROR many supertypes available

class Inv { operator fun invoke() = "" }
open class Invokable { val fn = Inv() }
class C3 : Invokable(), I { fun foo() = super.fn() } // ERROR many supertypes available

open class Private { private val fn = { "x" } }
class C4 : Private(), I { fun foo() = super.fn() } // ERROR many supertypes available

interface WithGetter { val fn: () -> String get() = { "" } }
open class Fun { open fun fn(): String = "" }
class C5 : Fun(), WithGetter { fun foo() = super.fn() } // ERROR many supertypes available

interface Abstract { val fn: () -> String }
abstract class C6 : Fun(), Abstract { fun foo() = super.fn() }

class C7 : JB(), I { fun foo() = super.fn() }

open class Both {
    val bar = { "bar.invoke()" }
    open fun bar(): String = "bar()"
}
class C8 : Both(), I { override fun bar() = super.bar() }

interface Other { val other: Int get() = 1 }
class C9 : Lambda(), Other { fun foo() = super.fn() }
