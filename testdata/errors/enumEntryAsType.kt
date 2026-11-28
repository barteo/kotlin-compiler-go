// An enum entry isn't a type: kotlinc's ENUM_ENTRY_AS_TYPE, and of an is
// check, IS_ENUM_ENTRY.

enum class Color { RED, GREEN }

fun f(c: Color.RED) {} // ERROR use of enum entry names as types is prohibited

fun g(c: Color): Boolean = c is Color.GREEN // ERROR 'is' over enum entry is prohibited

fun h(): Array<Color.RED>? = null // ERROR use of enum entry names as types is prohibited
