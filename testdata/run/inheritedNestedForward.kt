// A nested class inherited through a class declared later in the file:
// Sort's supertype Settings is resolved after SortGroup's supertypes would
// be, so looking up Group resolves Settings' supertypes first.

open class Base {
    interface Group {
        val header: String?
    }
}

class Sheet {
    inner class Sort : Settings() {
        inner class SortGroup : Group {
            override val header = "sort"
        }
    }

    open inner class Settings : Base()
}

fun main() {
    val sort = Sheet().Sort()
    val group: Base.Group = sort.SortGroup()
    println(group.header)
}
