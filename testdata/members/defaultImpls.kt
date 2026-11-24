// The members that call the bodies an interface or a class inherits from
// a generic interface, with the type arguments of the supertype, and
// those of member extensions (TestMembers).
package d

class Fin

interface Cond<T> {
    fun test(t: T): Boolean = true
    fun get(): T? = null
    fun both(a: T, n: Int = 1): T = a
    fun List<T>.ext(): Int = size
    fun T.ext2(): Int = 1
    val prop: T? get() = null
    var T.extProp: Int
        get() = 1
        set(v) {}
    fun plain(s: String): String = s
}

interface FinCond : Cond<Fin>
interface IntCond : Cond<Int>
interface Mid<U> : Cond<List<U>>
interface StrMid : Mid<String>
interface Renamed<V : Fin> : Cond<V>

class Impl : FinCond
class StrImpl : StrMid
abstract class Abs<V : Any> : Cond<V>
class Conc : Abs<Fin>()
open class Open : FinCond
class Derived : Open()
class Overrides : FinCond { override fun test(t: Fin): Boolean = false }

interface Base { fun String.ext(): Int = length }
interface Sub : Base
interface SubSub : Sub { fun Int.ext(): Int = this }
class ExtImpl : Sub
class ExtImpl2 : SubSub
class ExtOwn : Sub { override fun String.ext(): Int = 0 }
