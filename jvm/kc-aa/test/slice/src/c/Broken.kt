package c

import a.Marker

// A syntax error drops only the member it is in.
@Marker class Broken {
    @Marker fun kept() {}
    fun broken( =
}

@Gone class Unresolved {
    @Marker val p: Missing = TODO()
}
