package lib

@RequiresOptIn
annotation class Exp

@RequiresOptIn
annotation class Other

@Exp
fun expFun() = 1

@Other
fun otherFun() = 1
