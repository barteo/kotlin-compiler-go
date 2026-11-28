@file:OptIn(ExperimentalContracts::class)

// Smart casts of a var that a lambda or a local function assigns, as K2
// has them: of the outermost of the frames entered since the var's
// declaration, unless it is the only one and runs in place, every
// assignment of it may come between a check and a read, and must have
// been checked before the read and keep the smart cast's type.

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.InvocationKind
import kotlin.contracts.contract

fun exactlyOnce(block: () -> Unit) {
    contract { callsInPlace(block, InvocationKind.EXACTLY_ONCE) }
    block()
}

fun later(block: () -> Unit) {}

fun nestedInPlace(items: List<List<Int>>) {
    var acc: Any = 0
    items.forEach { a ->
        a.forEach { b ->
            if (acc is String) acc.length // ERROR smart cast to 'String' is impossible
            acc = b
        }
    }
}

fun outerAssigns() {
    var x: Any = ""
    exactlyOnce {
        exactlyOnce { if (x is String) x.length } // ERROR smart cast to 'String' is impossible
        x = "later, unchecked when read"
    }
}

fun notInPlace() {
    var x: Any = ""
    later {
        if (x is String) x.length // ERROR smart cast to 'String' is impossible
        x = ""
    }
}

fun localFunction() {
    var x: Any = ""
    require(x is String)
    fun local() {
        exactlyOnce { x.length } // ERROR smart cast to 'String' is impossible
        x = ""
    }
}

fun compoundAssignment() {
    var x: Any = ""
    later {
        if (x is String) x.length // ERROR smart cast to 'String' is impossible
        x += "" // ERROR operator '+=' cannot be applied to Any
    }
}

// These keep their smart casts: an only frame that runs in place, an
// assignment checked before the read that keeps the type, and x++, which
// isn't one here.
fun onlyInPlace(items: List<Int>) {
    var acc: Any = 0
    items.forEach { a ->
        if (acc is String) acc.length
        acc = a
    }
}

fun checkedBefore() {
    var x: Any = ""
    later {
        x = ""
        if (x is String) x.length
    }
}

fun increments() {
    var x: Int? = 0
    later {
        if (x != null) {
            x.inc()
            x++
        }
    }
}

fun noAssignment() {
    var x: Any = ""
    require(x is String)
    fun local() {
        exactlyOnce { x.length }
    }
}
