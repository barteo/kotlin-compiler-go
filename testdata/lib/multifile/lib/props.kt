// A third part, with properties: each part holds its fields, accessors
// and static initializer, and the facade's accessors call the part's.
@file:JvmMultifileClass
@file:JvmName("Text")

package mf

val processors: Int = "1234".length

var counter = 0

internal val tag = "t" + processors

val doubled: Int
    get() = processors * 2

var settable: String = "a"
    set(value) {
        field = value + "!"
    }

val lazyName: String by lazy { "lazy" + counter }

const val LIMIT = 9

fun bump(): Int {
    counter += processors
    return counter
}

@PublishedApi
internal val published = 5

inline fun withPublished(): Int = published + LIMIT
