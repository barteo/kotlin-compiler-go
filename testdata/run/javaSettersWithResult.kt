import java.nio.file.attribute.FileTime
import java.util.concurrent.RecursiveAction
import java.util.zip.ZipEntry

// A Java setter that returns a value is a property's setter too, as
// Android's Intent.setFlags: the assignment drops the value.
class Task : RecursiveAction() {
    override fun compute() {}
}

fun <T> T.edit(f: T.() -> Unit) = f()

fun main() {
    val e = ZipEntry("a")
    e.lastModifiedTime = FileTime.fromMillis(86400000L)
    println(e.lastModifiedTime.toMillis())
    e.edit {
        creationTime = FileTime.fromMillis(1000L)
    }
    println(e.creationTime.toMillis())

    val t = Task()
    t.forkJoinTaskTag = 3
    t.forkJoinTaskTag++
    t.forkJoinTaskTag = (t.forkJoinTaskTag + 2).toShort()
    println(t.forkJoinTaskTag)
}
