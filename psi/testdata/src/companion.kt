// Companion blocks (Kotlin 2.4 PSI).
class A {
    companion {
        val x = 1
        fun f() = x
    }

    companion object {
        val y = 2
    }

    companion object Named {}

    private companion {
    }

    companion
    {
    }
}

object O {
    companion { }
}
