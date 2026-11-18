package d

annotation class Platform(val name: String = "")

// Expect declarations without actual ones: a processor generates those.
@Platform("clock")
expect class Clock(zone: String) {
    val zone: String
    fun now(): Long
    companion object {
        fun system(): Clock
    }
}

expect interface Feed {
    fun read(): String
}

expect object Registry : Feed {
    override fun read(): String
}

@Platform
expect fun clock(zone: String = "UTC"): Clock

expect val defaultClock: Clock

class Timer(val clock: Clock, val feed: Feed = Registry) {
    fun start(): Long = clock.now()
}

expect class Bare {
    fun f()
}

expect class Made {
    constructor(n: Int)
    var n: Int
}

expect annotation class Tag(val text: String)
