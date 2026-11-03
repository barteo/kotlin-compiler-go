// Typealiases that another package imports: nested in a class, and one
// whose enum's entries a star import through it imports.
@file:Suppress("TOPLEVEL_TYPEALIASES_ONLY")

package defs

enum class Color { RED, GREEN }

typealias Colors = Color

class Outer {
    typealias Name = String
}
