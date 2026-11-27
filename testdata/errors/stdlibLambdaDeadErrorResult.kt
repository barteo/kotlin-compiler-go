fun t1(x: Any) {
    run { // ERROR cannot infer
        try { } finally { return }
        x.bbb() // ERROR unresolved reference: bbb
    }
}

fun t2(x: Any) {
    run { // ERROR cannot infer
        if (x is String) return else return
        x.bbb() // ERROR unresolved reference: bbb
    }
}

fun t3(x: Any): Int {
    val n = run {
        if (x is String) return 1 else return 2
        x.hashCode()
    }
    return n
}

fun t4(x: Any) {
    run {
        return
        x.toString()
    }
}
