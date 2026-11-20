// JAVA_CLASS_ON_COMPANION: A.javaClass, of a class A named for its
// companion object, is the companion's class.
package test

import test.A2 as B

class A {
    companion object
    fun f() = javaClass
    fun g() = Companion.javaClass
    fun h() = A.javaClass // JAVA_CLASS_ON_COMPANION
}

class A2 {
    companion object
}

class N {
    companion object Named
}

typealias T = A

class G<T> {
    companion object
}

object O

enum class E { ENTRY }

val a0 = A.javaClass // JAVA_CLASS_ON_COMPANION
val a1 = T.javaClass // JAVA_CLASS_ON_COMPANION
val a2 = test.A.javaClass // JAVA_CLASS_ON_COMPANION
val a3 = B.javaClass // JAVA_CLASS_ON_COMPANION
val a4 = A.Companion.javaClass
val a5 = A.Companion.let { it.javaClass }
val a6 = A::class.java
val n0 = N.javaClass // JAVA_CLASS_ON_COMPANION
val n1 = N.Named.javaClass
val o0 = O.javaClass
val e0 = E.ENTRY.javaClass
val int0 = Int.javaClass // JAVA_CLASS_ON_COMPANION
val int1 = Int.Companion.javaClass
val string0 = kotlin.String.javaClass // JAVA_CLASS_ON_COMPANION
val g0 = G.javaClass // JAVA_CLASS_ON_COMPANION
@Suppress("JAVA_CLASS_ON_COMPANION")
val s0 = A.javaClass
