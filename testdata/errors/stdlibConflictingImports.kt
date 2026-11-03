// Two explicit imports of classes or type aliases of one name conflict,
// even of one class; functions and properties of one name are overloads.
import kotlin.Throws // ERROR conflicting import: imported name 'Throws' is ambiguous
import kotlin.jvm.Throws // ERROR conflicting import: imported name 'Throws' is ambiguous
import java.util.Random // ERROR conflicting import: imported name 'Random' is ambiguous
import java.util.Random // ERROR conflicting import: imported name 'Random' is ambiguous
import java.util.Date as D // ERROR conflicting import: imported name 'Date' is ambiguous
import java.sql.Date as D // ERROR conflicting import: imported name 'Date' is ambiguous
import java.util.Map.Entry // ERROR conflicting import: imported name 'Entry' is ambiguous
import kotlin.collections.Map.Entry // ERROR conflicting import: imported name 'Entry' is ambiguous
import kotlin.math.max
import kotlin.math.max
import kotlin.math.PI
import kotlin.math.PI
import java.util.List as JList
import kotlin.collections.List
import java.util.*
import java.util.*

fun use(): Double = max(PI, 1.0)
