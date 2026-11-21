const val int = 456
const val equal1 = 1 === 2 // ERROR const 'val' initializer should be a constant value
const val equal2 = int !== 2 // ERROR const 'val' initializer should be a constant value
const val ok = int == 2
