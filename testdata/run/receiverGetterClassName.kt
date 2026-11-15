// A capitalized name in a scope whose implicit receiver has a Java getter
// of that name, as TimeZone with a SimpleDateFormat's getTimeZone(), is
// the class: Kotlin sees the getter as the property timeZone.
import java.text.SimpleDateFormat
import java.util.Locale
import java.util.TimeZone

fun SimpleDateFormat.inGmt(): SimpleDateFormat {
    timeZone = TimeZone.getTimeZone("GMT")
    return this
}

inline fun <T> T.configure(block: T.() -> Unit): T {
    block()
    return this
}

fun main() {
    val f = SimpleDateFormat("HH:mm", Locale.US).inGmt()
    println(f.timeZone.id)
    println(f.format(java.util.Date(3_600_000L)))
    val g = SimpleDateFormat("HH:mm", Locale.US).configure {
        timeZone = TimeZone.getTimeZone("GMT+2")
    }
    println(g.format(java.util.Date(0L)))
}
