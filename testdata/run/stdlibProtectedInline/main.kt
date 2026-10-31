package app

import pool.Pool

class Names : Pool<String>() {
    private var n = 0
    override fun make() = "s${n++}"

    fun fill(k: Int) {
        relabel { "$it-names" }
        repeat(k - 1) { relabel { it } }
    }

    fun all(): List<String> {
        val out = ArrayList<String>()
        forEachSlot { out += it }
        return out
    }

    fun first(): String? {
        forEachSlot { return it }
        return null
    }
}

fun main() {
    val p = Names()
    println(p.all())
    println(p.first())
    p.fill(3)
    println(p.all())
    println(p.first())
    println(p)
}
