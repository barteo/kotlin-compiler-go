package d.sub

import d.Source

abstract class Partial : Source<Long> {
    override val name: String get() = "partial"
}
