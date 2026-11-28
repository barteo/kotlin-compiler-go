// Smart casts through locals that hold the same value, as kotlinc 2.4.21
// makes them: of the same property of an alias, and through a Boolean
// local that holds a condition's val.

class AnyHolder(val value: Any?)
class StringHolder(val value: String?)

fun propertyOfAlias(x: AnyHolder) {
    var a = x
    val b = a
    if (a.value is String) x.value.length
    if (b.value is String) x.value.length
    if (x.value is String) b.value.length
    val f = {
        if (a.value is String) x.value.length // ERROR unresolved reference: length
        if (b.value is String) x.value.length
    }
    a = AnyHolder("")
    f()
}

fun notNullOfAlias(x: StringHolder) {
    val a = x
    if (a.value != null) x.value.length
    if (x.value == null) return
    a.value.length
}

fun conditionOfAlias(x: String?) {
    val a = x != null
    var b = a
    val c = b
    if (b) x.length
    if (c) x.length
    val f = {
        if (b) x.length // ERROR nullable receiver
        if (c) x.length
    }
    b = true
    f()
}

fun conditionOfReassigned(x: String?) {
    val a = x != null
    var b = a
    b = true
    if (b) x.length // ERROR nullable receiver
}
