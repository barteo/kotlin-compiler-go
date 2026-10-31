package p.q

import p.q.missing // ERROR unresolved import: p.q.missing
import nowhere.* // ERROR unresolved import: nowhere.*

fun f(): Int = 1

fun g(): Int = p.q.f() + p.q.h() // ERROR unresolved reference: h

val x: Int = p.q.y // ERROR unresolved reference: y
