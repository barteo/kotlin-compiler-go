package store

// A copy of written in another file than its own creates objects of its
// classes, which its file generates.
suspend fun greeting(): String = written { it.write("hi") }
