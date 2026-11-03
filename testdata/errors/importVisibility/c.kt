package c

import a.Hidden // ERROR cannot access 'Hidden': it is private in file
import a.HiddenAlias
import a.hidden // ERROR cannot access 'hidden': it is private in file
import a.overloaded
import a.hiddenVal // ERROR cannot access 'hiddenVal': it is private in file
import a.Visible.Nested

fun use() {
    val n: Nested? = null // ERROR cannot access 'Visible.Nested': it is private in 'Visible'
}

// A private typealias is reported where it is used.
fun alias() {
    val x: HiddenAlias = 1 // ERROR cannot access 'HiddenAlias': it is private in file
}
