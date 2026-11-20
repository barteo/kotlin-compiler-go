// The DEPRECATION warnings below, as TestWarningsDeprecation reads them:
// each line ends with a comment naming the warnings it reports, if any.
// They follow kotlinc 2.4.21's FirDeprecationChecker and the tests
// propertyUsage.kt, deprecatedAll.kt, propertyUseSiteTargetedAnnotations.kt,
// typealiasCompanionObject.kt, annotationUsage.kt and
// staticImportDeprecatedReceiver.kt of its diagnostics tests.
@file:Suppress("UNUSED_PARAMETER", "UNUSED_VARIABLE")

package dep

import dep.Old.Companion.a // DEPRECATION
import dep.Old

@Deprecated("old")
class Old {
    companion object {
        val a = ""
    }
}

class Holder {
    var test2: String = ""
        @Deprecated("get") get
        @Deprecated("set") set

    var test4: String = ""
        @Deprecated("set") set

    @set:Deprecated("set")
    var withSetter: String = ""

    @Deprecated("p")
    var name = ""

    @all:Deprecated("all")
    var everything = 1
}

fun Holder.ext() {
    test2 = "ext" // DEPRECATION
    test4
    test4 = "" // DEPRECATION
}

fun writes(h: Holder) {
    h.test4
    h.test4 = "" // DEPRECATION
    h.withSetter
    h.withSetter = "" // DEPRECATION
    h.name = "" // DEPRECATION
    h.everything // DEPRECATION
    h.everything = 2 // DEPRECATION
    h.everything++ // DEPRECATION
    h.everything += 1 // DEPRECATION
}

@Deprecated("x")
fun inDeprecated(h: Holder) {
    h.name = ""
}

fun chars(c: Char) {
    c.toInt() // DEPRECATION
    c.toLong() // DEPRECATION
    c.toDouble() // DEPRECATION
    c.code
    'a'.toByte() // DEPRECATION
}

@Deprecated("text")
annotation class Obsolete

@Obsolete // DEPRECATION
class UsesObsolete

class Relevant {
    companion object {
        val value = ""
    }
}

@Deprecated("Use Relevant")
typealias OldRelevant = Relevant

fun aliasObject() = OldRelevant // DEPRECATION
fun aliasMember() = OldRelevant.value // DEPRECATION

enum class E {
    A,
    @Deprecated("b") B,
}

fun entries() {
    E.A
    E.B // DEPRECATION
}
