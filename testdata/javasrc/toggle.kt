import widget.Fancy
import widget.Widget
import widget.verticalPadding

// A Kotlin subclass that overrides Java setters, and assigns its Java
// and extension properties as this's.
class Toggle : Widget() {
    val log = mutableListOf<String>()

    var state: Int = 0
        set(value) {
            field = value
            isEnabled = value > 0
            when (value) {
                1 -> isEnabled = true
                else -> isEnabled = false
            }
        }

    init {
        verticalPadding = 3
        lineHeight = 4
    }

    override fun setEnabled(enabled: Boolean) {
        super.setEnabled(enabled)
        log += "enabled $enabled"
    }

    override fun setLineHeight(h: Int) {
        super.setLineHeight(h)
        log += "lineHeight $h"
    }
}

class Builder(val listener: ((Widget) -> Unit)?)

fun widgets() {
    val t = Toggle()
    t.state = 1
    t.state = 0
    println(t.log)
    println("${t.isEnabled} ${t.lineHeight} ${t.verticalPadding}")
    val w = Widget()
    w.filters = arrayOf(Widget.Filter { it })
    val fs: Array<out Widget.Filter> = w.filters
    w.filters = fs
    w.filters = arrayOfNulls(0)
    w.items = listOf("a")
    println("${fs.size} ${w.filters.size} ${w.items}")
    val f: (Widget) -> Unit = { println("listener") }
    w.setOnDismissListener(f)
    println(w.dismiss())
    w.setOnDismissListener(Builder(null).listener)
    println(w.dismiss())
    w.setOnDismissListener(Builder { println("builder's") }.listener)
    println(w.dismiss())
    println(Fancy.getInstance().fancy())
}
