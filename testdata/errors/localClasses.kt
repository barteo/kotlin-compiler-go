fun f() {
    class A {
        class Nested // ERROR 'Class' is prohibited here
        inner class Inner {
            object O // ERROR named object 'O' cannot be local
        }
    }
    object {
        interface I // ERROR 'Interface' is prohibited here
    }
}

inline fun g() {
    class InInline // ERROR local classes are not yet supported in inline functions
}
