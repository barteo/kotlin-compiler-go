// A fourth part, with lateinit properties, whose fields the part holds:
// another module reads and writes them through the facade's accessors.
@file:JvmMultifileClass
@file:JvmName("Text")

package mf

lateinit var late: String

lateinit var lateOwner: String
    private set

fun lateInit(v: String) {
    lateOwner = v
}

fun lateReady() = ::late.isInitialized
