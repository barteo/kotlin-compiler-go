import p.Settings
import p.describe

fun main() {
    val s = Settings().apply {
        setTaskIds(listOf(1L, 2L))
        parentId = 3L
    }
    println("${describe(s)} ${s.parentId}")
    with(s) { setTaskIds(setOf(4L)) }
    println(s.taskIds)
}
