package time

import base.*
import java.time.Duration

// The package's own functions don't take a Long: the star import's do.
fun delay(duration: Duration): Unit = delay(duration.toMillis())
fun <T> List<T>.debounce(timeout: Duration): List<T> = debounce(timeout.toMillis())

fun run() {
    delay(Duration.ofSeconds(1))
    delay(5)
    listOf(1).debounce(Duration.ofMillis(5))
}
