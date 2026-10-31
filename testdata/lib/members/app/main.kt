import p.Name
import p.Name.Companion.shout
import p.Point
import p.Tag
import p.VERSION
import p.ext
import p.label
import p.size

fun main() {
    val t = p.sameTag()
    println(t.label)
    t.label = "new"
    println(t.text + " " + t.size + " " + VERSION)
    println(Name("abc").shout())
    println(q.useBoth(Tag("z")))
    val pt = Point(1, 2)
    println("${pt.component2()} ${pt.sum()}")
    println(ext())
}
