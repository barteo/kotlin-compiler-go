@file:OptIn(kotlin.contracts.ExperimentalContracts::class, kotlin.contracts.ExperimentalExtendedContracts::class)

import kotlin.contracts.*

fun both(a: String?, b: Boolean): String? {
    contract { // ERROR only a 'null' or 'is' check can imply returnsNotNull()
        (a != null && b) implies returnsNotNull()
    }
    return if (a != null && b) a else null
}

fun decode(encoded: String?): String? {
    contract { (encoded != null) implies (returnsNotNull()) }
    return encoded
}

val Any?.isStr: Boolean
    get() {
        contract { returns(true) implies (this@isStr is String) }
        return this is String
    }

class Box(val v: Any?)

operator fun Box.contains(x: Any?): Boolean {
    contract { returns(true) implies (x is Int) }
    return x == v
}

fun use(s: String?, o: Any?, b: Box) {
    decode(s).length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed
    if (!o.isStr) o.length // ERROR on a nullable receiver of type Any?
    if (o !in b) o + 1 // ERROR on a nullable receiver of type Any?
}
