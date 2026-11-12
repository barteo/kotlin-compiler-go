package b

import a.Marker
import a.Other

@Marker
object Registry

@Marker
fun top() {}

enum class E { A, @Marker B }

@Other("y")
interface Plain
