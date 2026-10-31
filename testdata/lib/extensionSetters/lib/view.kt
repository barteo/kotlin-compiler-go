// A var extension property whose setter has the JVM name of a member of
// its receiver's class, as an Android TextView's textColor and its
// setTextColor(Int).
package view

class View {
    var color = 0

    fun setTextColor(c: Int) {
        color = c
    }
}

var View.textColor: Int
    get() = throw IllegalArgumentException("no getter")
    set(value) = setTextColor(value * 10)
