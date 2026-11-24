// The warnings of the deprecated inline classes below, as
// TestWarningsInlineClass reads them: each line ends with a comment
// naming the warnings it reports, if any. They follow the tests
// inlineClassDeprecated.kt and inlineClassDeclaration.kt of kotlinc
// 2.4.21's diagnostics tests.
@file:Suppress("UNUSED_VARIABLE")

inline class IC(val i: Int) // INLINE_CLASS_DEPRECATED

internal inline class Internal(val s: String) // INLINE_CLASS_DEPRECATED

@JvmInline
value class VC(val i: Int)

fun use(x: IC): Int = x.i
