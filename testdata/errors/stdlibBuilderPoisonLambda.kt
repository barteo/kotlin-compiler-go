fun <T> build(block: MutableList<T>.() -> Unit): List<T> = TODO()
fun tk(i: Int) {}
fun tk2(i: Int, f: () -> Unit) {}
fun <X> X.app(f: X.() -> Unit): X = this
fun <X> X.idd(): X = this
fun <Z> one(z: Z): Z = z
fun <R> scope(block: () -> R): R = block()
fun unit() {}

// As K2, a call of a generic extension with a lambda on an unresolved
// receiver leaves its error in the constraint system that the calls of
// a builder's lambda share: the builder's variables aren't inferred, and
// the calls after it that are checked with that system have the error.
fun poisoned(k: Int) {
    build { // ERROR cannot infer
        tk2(k) {}
        val a = Unknown234(1).app { // ERROR unresolved reference: Unknown234
            tk2(k) {} // ERROR argument type mismatch
        }
        // Checked on their own.
        tk(k)
        one(k)
        listOf(k)
        clear()
        // With a lambda, an anonymous function or a callable reference.
        tk2(k) {} // ERROR argument type mismatch
        IntArray(3) { 1 } // ERROR argument type mismatch
        tk2(k, fun() {}) // ERROR argument type mismatch
        tk2(k, ::unit) // ERROR argument type mismatch
        if (k > 1) tk2(k) {} // ERROR argument type mismatch
        fun local() { tk2(k) {} } // ERROR argument type mismatch
        // Members of the builder's receiver.
        add(1) // ERROR argument type mismatch
        this.add("") // ERROR argument type mismatch
        get(0) // ERROR argument type mismatch
        // A result that the call can't infer the type of, and its uses.
        val s = scope { k } // ERROR cannot infer type for type parameter 'R'
        tk(s) // ERROR argument type mismatch
        tk(one(s)) // ERROR argument type mismatch
        val d: Int = scope { k } // ERROR cannot infer type for type parameter 'R'
        tk(d)
        // The result of the call with the unresolved reference.
        a.foo() // ERROR unresolved reference: foo
        a[1] = 2 // ERROR unresolved reference: set
    }
}

// Not without a lambda.
fun notPoisoned(k: Int) {
    build {
        Unknown234(1).idd() // ERROR unresolved reference: Unknown234
        tk2(k) {}
        add("")
        scope { k }
    }
}

// What the lambda recorded before the unresolved reference still infers
// the builder's variables.
fun inferredBefore(k: Int) {
    val l = build {
        add("")
        Unknown234(1).app { } // ERROR unresolved reference: Unknown234
        add("") // ERROR argument type mismatch
        add(1) // ERROR argument type mismatch
        tk2(k) {} // ERROR argument type mismatch
    }
    val s: List<String> = l
    val m = build {
        add("")
        if (k > 0) {
            Unknown234(1).app { } // ERROR unresolved reference: Unknown234
        }
        add(1) // ERROR argument type mismatch
    }
    val t: List<String> = m
}
