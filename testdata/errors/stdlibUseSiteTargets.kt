@file:JvmName("UseSiteTargets")
@file:Suppress("UNUSED_VARIABLE")

package usesite

import kotlin.reflect.KProperty

annotation class Ann
annotation class Rep(val x: Int)

class Delegate {
    operator fun getValue(thisRef: Any?, prop: KProperty<*>): String = prop.name
    operator fun setValue(thisRef: Any?, prop: KProperty<*>, value: String) {}
}

class C(@param:Ann val a: String, @param:Ann b: Int, @field:Ann val c: Int) {
    @field:Ann val withField: Int = 1
    @field:Ann var usesField: Int = 1
        get() = field
        set(v) { field = v }
    @field:Ann lateinit var late: String
    @field:Ann // ERROR could be applied only to properties with backing fields
    val delegated: String by Delegate()
    @field:Ann // ERROR not applicable to target
    val noField: Int
        get() = 5
    val getterTarget: Int
        @get:Ann get() = 5 // ERROR '@get:' annotations could be applied only to property declarations
    @get:Ann @set:Ann @setparam:Ann @property:Ann var ok: String by Delegate()

    @param:Ann // ERROR '@param:' annotations could be applied only to primary constructor parameters
    val notParam: Int = 1

    @param:Ann // ERROR '@param:' annotations could be applied only to primary constructor parameters
    fun f(@param:Ann x: Int) { // ERROR '@param:' annotations could be applied only to primary constructor parameters
        @field:Ann // ERROR not applicable to target 'local variable'
        val l1 = 1
        @get:Ann // ERROR '@get:' annotations could be applied only to property declarations
        val l2 = 2
        @receiver:Ann // ERROR not applicable to target 'local variable'
        val l3 = 3
        @Ann
        val l4 = 4
    }

    @setparam:Ann // ERROR not applicable to target
    fun g() {}

    constructor(@param:Ann s: String) : this(s, 1, 1) // ERROR '@param:' annotations could be applied only to primary constructor parameters
}

@file:Ann // ERROR '@file:' annotations could be applied only to the file
fun h() {}

@setparam:Rep(1) // ERROR this annotation is not repeatable
var r = " "
    set(@Rep(2) x) {}
