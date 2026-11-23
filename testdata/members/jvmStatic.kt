// @JvmStatic on a property of an object or a companion object, or on one
// of its accessors alone: the static methods of the object's class, or
// of the companion's outer class (TestMembers).
package s

class A {
    companion object {
        @JvmStatic var whole: Int = 1
        var x: Int = 1
            @JvmStatic set
        @set:JvmStatic var y: String = ""
        @get:JvmStatic var g: Int = 1
        var h: Int = 1
            @JvmStatic get
        var customGet: Int = 1
            @JvmStatic get() = field + 1
        var customSet: Int = 1
            @JvmStatic set(v) { field = v + 1 }
        @get:JvmStatic val readOnly: Int = 1
        @get:JvmStatic @set:JvmStatic var both: Int = 1
        @set:JvmStatic var protectedSet: Int = 1
            protected set
        @get:JvmStatic val noField: Int get() = 3
        var plain: Int = 1
    }
}

object O {
    var x: Int = 1
        @JvmStatic set
    @get:JvmStatic var g: Int = 1
    @JvmStatic var whole: Int = 1
    @set:JvmStatic var customSet: Int = 1
        set(v) { field = v }
    var plain: Int = 1
}

interface I {
    companion object {
        @get:JvmStatic val g: Int = 1
        @set:JvmStatic var x: Int = 1
    }
}
