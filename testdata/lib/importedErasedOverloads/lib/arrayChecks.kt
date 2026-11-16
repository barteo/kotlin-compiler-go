package lib

fun Check<Array<*>>.holds(vararg items: Any?): String = "array ${actual.contentEquals(items)}"
