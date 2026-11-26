package use

import base.A
import base.C

open class Mid : A() {
    private val f = ""
}

class Sub : Mid() {
    fun bar() = f // ERROR the Java field 'f' is shadowed by the Kotlin property of 'use/Mid'
}

class Direct : A() {
    fun bar() = f
}
