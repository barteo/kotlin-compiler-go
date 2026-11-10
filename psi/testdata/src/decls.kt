#!/usr/bin/env kotlin
@file:Suppress("UNUSED") // trailing
@file:JvmName("Decls")

/**
 * File-level KDoc.
 */
package com.example.`decls`

import kotlin.math.*
import java.util.List as JList
import a.b.c

/** Doc for C. */
// plain comment
@Deprecated("x")
public open class C<in T : Any, out R>(val a: Int, private var b: String = "") : Base(a), I by impl where T : Comparable<T> {
    // member comment
    constructor(x: Long) : this(x.toInt())
    init { println(a) }
    internal val p: Int get() = 1
    var q: String = ""
        private set(value) { field = value }
    val lazyV by lazy { 42 }
    lateinit var w: String
    companion object Factory : Creator<C<*, *>> {
        const val K = 1
    }
    inner class Inner
    enum class E(val v: Int) { A(1), B(2) { override fun f() = 3 }, ; open fun f() = v }
    sealed interface S
    data class D(val x: Int, val y: Int)
    object O
    abstract fun abs(): Unit
    override fun toString(): String = "C"
    operator fun plus(o: C<T, R>): C<T, R> = this
    infix fun to(o: Any) = Pair(this, o)
    suspend inline fun <reified X> g(noinline f: () -> Unit, crossinline h: suspend X.(Int) -> String?) {}
    tailrec fun t(n: Int): Int = if (n == 0) 0 else t(n - 1)
    fun vararg(vararg xs: Int, ys: Array<out Number>) {}
    @get:JvmName("getZ") @set:JvmName("setZ") var z = 0
    private fun <T> T.ext(): T where T : CharSequence = this
    val Int.extProp: Int get() = this
    fun interface Fn { fun invoke() }
    value class V(val x: Int)
    expect fun e()
    context(Ctx) fun withCtx() {}
}

typealias M<K> = Map<K, List<K?>>
typealias F = suspend (Int, String) -> Unit
typealias G = Int.() -> Unit
fun interface Runner { fun run() }
val (a, b) = 1 to 2
private const val TOP = "top" /* trailing block */
var dyn: dynamic = null
fun ((Int) -> Unit).weird() {}
external fun nat(): Int
annotation class Ann(val value: String, vararg val more: Int)
@Ann("a", 1, 2) fun annotated(@Ann("p") param: @Ann("t") Int) {}
fun defaults(x: Int = 1, y: String = "$x") = x
