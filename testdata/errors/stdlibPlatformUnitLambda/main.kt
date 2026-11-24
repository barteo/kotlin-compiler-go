fun nullableUnit(): Unit? = null

fun assigned() {
    A.foo = l@ {
        if ("0".hashCode() == 42) return@l
        ""
    }
    A.foo = l@ { return@l null } // ERROR Nothing? but Unit was expected
    A.foo = l@ { return@l nullableUnit() } // ERROR Unit? but Unit was expected
}

fun passed() {
    A.run l@ {
        if ("0".hashCode() == 42) return@l
        ""
    }
    A.run { "" }
    run<Unit?> l@ {
        if ("0".hashCode() == 42) return@l
        ""
    }
    run<Unit?> l@ {
        if ("0".hashCode() == 42) return@l
        return@l null // ERROR Nothing? but Unit was expected
    }
    run<Unit?> l@ { return@l null }
}

val v: () -> Unit? = l@ {
    if ("0".hashCode() == 42) return@l // ERROR this lambda must return a value of type Unit?
    ""
}
