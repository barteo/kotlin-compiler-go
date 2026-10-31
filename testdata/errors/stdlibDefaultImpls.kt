class NoStart(override val endInclusive: Int) : ClosedRange<Int> // ERROR does not implement abstract member 'start'

class Complete(override val start: Int, override val endInclusive: Int) : ClosedRange<Int>
