// An inline function of a local class or an object expression, private or
// not, isn't private to K2: it can't use a private class's members. Nor
// is it public API, which may use internal ones.

private class P {
    val x = 1
    fun inside() {
        object {
            private inline fun g() = x // ERROR cannot access members of private class
        }
    }
}

private fun f(p: P) {
    object {
        private inline fun g() = p.x // ERROR cannot access members of private class
        fun h() = p.x
    }
    class L {
        inline fun k() = p.x // ERROR cannot access members of private class
    }
}

class P2 { internal val x = 1 }

fun f2(p: P2) {
    class L {
        inline fun k() = p.x
    }
    object {
        inline fun g() = p.x
    }
}
