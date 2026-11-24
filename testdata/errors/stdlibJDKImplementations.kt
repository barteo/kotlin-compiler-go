// A class must override a method it inherits an implementation of from
// one of its interfaces and another from a JDK class, or from a JDK
// interface's default method: java.util.Map's replace(K, V), which an
// AbstractMutableMap<String, String> has, and IS's replace(String,
// String) are one member. A Kotlin function of an Int and the JDK's
// method of an Integer aren't, and a function size() isn't the property
// size of a collection.
import java.util.function.BiConsumer

abstract class MyMap<K : Any, V : Any> : AbstractMutableMap<K, V>()

interface II { fun replace(key: Int, value: Int): Int? = null }
interface IS { fun replace(key: String, value: String): String? = null }
interface IA { fun replace(key: String, value: String): String? }
interface IG { fun getOrDefault(key: String, defaultValue: String): String = "" }
interface IL { fun add(e: String): Boolean = true }
interface IF { fun forEach(action: BiConsumer<in String, in String>) {} }
interface IZ { fun size(): Int = 0 }
interface IK { fun keySet(): Set<String> = emptySet() }
interface IT { fun start() {} }
interface IR : Runnable { override fun run() {} }
interface IM : MutableMap<String, String> {
    override fun replace(key: String, value: String): String? = null
}

abstract class ZI : MyMap<Int, Int>(), II
abstract class ZS : MyMap<String, String>(), IS // ERROR must override 'replace'
abstract class ZO : MyMap<String, String>(), IS {
    override fun replace(key: String, value: String): String? = null
}
abstract class ZA : HashMap<String, String>(), IA
abstract class ZG : MyMap<String, String>(), IG // ERROR must override 'getOrDefault'
abstract class ZL : java.util.AbstractList<String>(), IL // ERROR must override 'add'
abstract class ZF : HashMap<String, String>(), IF // ERROR must override 'forEach'
abstract class ZM : HashMap<String, String>(), IM // ERROR must override 'replace'
abstract class ZZ : ArrayList<String>(), IZ
abstract class ZK : HashMap<String, String>(), IK
class ZT : Thread(), IT // ERROR must override 'start'
class ZR : Thread(), IR // ERROR must override 'run'

open class P : HashMap<String, String>() {
    override fun replace(key: String, value: String): String? = null
}
class ZP : P(), IS // ERROR must override 'replace'
