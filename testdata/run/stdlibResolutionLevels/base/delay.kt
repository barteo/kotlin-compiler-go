package base

fun delay(millis: Long) = println("base delay $millis")
fun delay(duration: kotlin.time.Duration) = println("base duration $duration")
fun <T> List<T>.debounce(millis: Long): List<T> {
    println("base debounce $millis")
    return this
}
