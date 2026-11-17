package lib

interface Units {
    fun String.tagged(times: Int): String = "<$this>".repeat(times)

    val Int.half: Int
        get() = this / 2
}

abstract class Base : Units
