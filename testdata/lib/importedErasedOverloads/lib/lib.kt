package lib

@JvmInline
value class Span(val millis: Long)

@JvmInline
value class Point(val packed: Long)

fun pause(millis: Long): String = "pause for $millis"

fun pause(span: Span): String = "pause for a span of ${span.millis}"

fun blend(a: Point, b: Point, t: Float): String = "points ${a.packed} ${b.packed} $t"

fun blend(a: Span, b: Span, t: Float): String = "spans ${a.millis} ${b.millis} $t"

fun blend(a: Long, b: Long, t: Float): String = "longs $a $b $t"

fun <T> List<T>.each(span: Span, action: (T) -> Unit) = forEach { action(it); println(pause(span)) }

fun <T> List<T>.each(millis: Long, action: (T) -> Unit) = forEach { action(it); println(pause(millis)) }
