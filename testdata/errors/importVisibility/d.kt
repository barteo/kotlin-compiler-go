package d

import a.*
import b.*

// a.Hidden is private to its file: the star import finds b.Hidden.
fun use(h: Hidden) = h.x
