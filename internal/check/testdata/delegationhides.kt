// DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE: delegation implements a
// member that another supertype overrides.

interface Base {
    fun test() = "Base"
    val prop: String get() = "Base"
}
interface Base2 : Base {
    override fun test() = "Base2"
}
interface Base3 : Base {
    override val prop: String get() = "Base3"
}
class Impl : Base
class Impl2 : Base2

class HidesFun : Base2, Base by Impl() // DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE
class HidesProp : Base3, Base by Impl() // DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE
class Subtype : Base, Base2 by Impl2()

abstract class FromClass : Base {
    override fun test() = "FromClass"
}
abstract class Middle : FromClass()
abstract class ThroughClass : Middle(), Base by Impl() // DELEGATED_MEMBER_HIDES_SUPERTYPE_OVERRIDE

abstract class Reabstracted : Base2 {
    abstract override fun test(): String
}
abstract class NotHidden : Reabstracted(), Base by Impl()

class Overrides : Base2, Base by Impl() {
    override fun test() = "Overrides"
}

interface ToDelegate { fun run() }
class Runner : ToDelegate { override fun run() {} }
abstract class Abstract : ToDelegate
abstract class OnlyAbstract : Abstract(), ToDelegate by Runner()
