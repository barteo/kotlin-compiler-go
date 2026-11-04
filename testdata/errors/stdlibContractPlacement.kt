@file:OptIn(kotlin.contracts.ExperimentalContracts::class)

import kotlin.contracts.*

// Contracts kotlinc doesn't allow, and the extended contract DSL without
// opt-in to ExperimentalExtendedContracts.

class A(var v: Int = 0) {
    override fun equals(other: Any?): Boolean {
        contract { returns(true) implies (other is A) } // ERROR contracts are not allowed for open or override functions
        return v == (other as? A)?.v
    }
}

operator fun A.compareTo(a: A?): Int {
    contract { returns() implies (a != null) } // ERROR contracts are not allowed for operator compareTo
    return v.compareTo(a!!.v)
}

operator fun A.plusAssign(body: () -> Int) {
    contract { callsInPlace(body, InvocationKind.EXACTLY_ONCE) } // ERROR contracts are not allowed for operator plusAssign
    v += body()
}

open class Host {
    open var b: String
        get() = ""
        set(value) {
            contract { returns() } // ERROR contracts are not allowed for open or override functions
        }
}

val Boolean.anon: Int
    get() = fun(): Int {
        contract { returns() implies (this@anon) } // ERROR contracts are not allowed for anonymous functions
        return 1
    }()

fun local() {
    fun g(x: Boolean) {
        contract { returns() implies (x) } // ERROR contracts are not allowed for local functions
    }
    g(true)
}

fun annotated(x: Any?): Boolean {
    contract {
        @Suppress("SENSELESS_COMPARISON") // ERROR annotations are not allowed in contract descriptions
        returns(true) implies (x != null)
    }
    return x != null
}

fun decode(encoded: String?): String? {
    contract {
        (encoded != null) implies (returnsNotNull()) // ERROR needs opt-in
    }
    return encoded
}

// kotlinc still reads the contracts it doesn't allow.
fun use(a1: A?, a2: A?) {
    if (A().equals(a1)) a1.v
    if (A() < a2) a2.v
}
