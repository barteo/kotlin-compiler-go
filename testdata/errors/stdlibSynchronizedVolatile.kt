// kotlinc's checks of @Synchronized, @Volatile and synchronized blocks:
// what the JVM can't synchronize, or a volatile field that can't be
// written, is an error.
import kotlin.concurrent.Volatile as CV
import kotlin.properties.Delegates

@JvmInline
value class V(val x: Int) {
    @Synchronized fun f() {} // ERROR '@Synchronized' annotation has no effect on value classes
    val p: Int @Synchronized get() = 1 // ERROR '@Synchronized' annotation has no effect on value classes
}

@JvmInline
value class W(@get:Synchronized val x: Int) // ERROR '@Synchronized' annotation has no effect on value classes

annotation class Ann(@get:Synchronized val x: Int) // ERROR '@Synchronized' annotation cannot be used on annotation parameters

class Ctor(@get:Synchronized val x: Int, @set:Synchronized var y: Int)

interface I {
    @Synchronized fun g() {} // ERROR '@Synchronized' annotation cannot be used on interface members
    @Synchronized fun h() // ERROR '@Synchronized' annotation cannot be used on interface members
    val p: Int @Synchronized get() = 1 // ERROR '@Synchronized' annotation cannot be used on interface members
}

abstract class A {
    @Synchronized abstract fun a() // ERROR '@Synchronized' annotation cannot be used on abstract functions
    @Synchronized suspend fun s() {} // ERROR '@Synchronized' annotation is not applicable to 'suspend' functions and lambdas
    @Synchronized abstract suspend fun sa() // ERROR '@Synchronized' annotation is not applicable to 'suspend' functions and lambdas
    @get:Synchronized abstract val p: Int // ERROR '@Synchronized' annotation cannot be used on abstract functions
    @Synchronized fun ok() {}
}

@Synchronized suspend fun top() {} // ERROR '@Synchronized' annotation is not applicable to 'suspend' functions and lambdas

class Vol {
    @Volatile val a = 1 // ERROR '@Volatile' annotation cannot be used on immutable properties
    @Volatile var b = 1
    @CV val d = 2 // ERROR '@Volatile' annotation cannot be used on immutable properties
    @field:Volatile val e = 3 // ERROR '@Volatile' annotation cannot be used on immutable properties
    @Volatile lateinit var g: String
    @kotlin.concurrent.Volatile var h = 1
    @Volatile private val pv = 1 // ERROR '@Volatile' annotation cannot be used on immutable properties
    @delegate:Volatile var pd by Delegates.observable(1) { _, _, _ -> } // ERROR '@Volatile' annotation cannot be used on delegated properties
    @Volatile val pd2 by lazy { 1 } // ERROR '@Volatile' annotation cannot be used on delegated properties
    @Volatile var withSetter = 1
        private set
}

class C(@Volatile val ctor: Int, @Volatile var ok: Int) // ERROR '@Volatile' annotation cannot be used on immutable properties

@Volatile val topVal = 1 // ERROR '@Volatile' annotation cannot be used on immutable properties

fun <T : Any> blocks(v: V, n: Int, s: String, l: Long?, i: Int?, t: T, a: Any, vn: V?, u: UInt) {
    synchronized(v) {} // ERROR synchronizing on 'V' is forbidden
    synchronized(n) {} // ERROR synchronizing on 'Int' is forbidden
    synchronized(1 + 2) {} // ERROR synchronizing on 'Int' is forbidden
    synchronized(l!!) {} // ERROR synchronizing on 'Long' is forbidden
    synchronized(i ?: 0) {} // ERROR synchronizing on 'Int' is forbidden
    synchronized(lock = 'c') {} // ERROR synchronizing on 'Char' is forbidden
    synchronized((true)) {} // ERROR synchronizing on 'Boolean' is forbidden
    synchronized(vn ?: v) {} // ERROR synchronizing on 'V' is forbidden
    synchronized(u) {} // ERROR synchronizing on 'UInt' is forbidden
    kotlin.synchronized(Result.success(1)) {} // ERROR synchronizing on 'Result<Int>' is forbidden
    synchronized(s) {}
    synchronized(t) {}
    synchronized(a) {}
    if (i != null) synchronized(i) {} // ERROR synchronizing on 'Int' is forbidden
    @Synchronized suspend fun local() {} // ERROR '@Synchronized' annotation is not applicable to 'suspend' functions and lambdas
    val sl: suspend () -> Unit = @Synchronized {} // ERROR '@Synchronized' annotation is not applicable to 'suspend' functions and lambdas
    val sl2 = suspend @Synchronized {} // ERROR '@Synchronized' annotation is not applicable to 'suspend' functions and lambdas
    val plain = @Synchronized {}
}
