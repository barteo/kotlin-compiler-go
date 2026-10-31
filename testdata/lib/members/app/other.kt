package q

import p.Tag
import p.render

// An imported extension that doesn't fit the receiver gives way to the
// package's own.
class Local

fun Local.render(s: String) = "app $s"

fun useBoth(t: Tag) = t.render(1) + ", " + Local().render("x")
