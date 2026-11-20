// CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING: an inherited var whose setter is
// less visible than an abstract property's it implements.
open class A1 {
    var a: Int = 10
        protected set
}
interface I1 {
    var a: Int
}
abstract class B1 : A1(), I1 // CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING
abstract class WithVarPrivateSet {
    var z: Int = 42
        private set
}
interface IVar {
    var z: Int
}
class G2 : WithVarPrivateSet(), IVar // CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING
open class A3 {
    var a: Int = 10
        internal set
}
abstract class B3 : A3(), I1 // CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING
open class A4 {
    var a: Int = 10
}
abstract class B4 : A4(), I1
interface IVal {
    val z: Int
}
class G5 : WithVarPrivateSet(), IVal
interface IProt {
    var a: Int
}
@Suppress("CANNOT_WEAKEN_ACCESS_PRIVILEGE_WARNING")
abstract class B6 : A1(), IProt
