typealias TAStatic = JvmStatic

class A {
    companion object {
        @JvmStatic const val z = 1 // ERROR '@JvmStatic' annotation is useless for const or '@JvmField' properties
        @JvmStatic @JvmField val x = 1 // ERROR '@JvmStatic' annotation is useless for const or '@JvmField' properties
        @JvmStatic val ok = 1
    }
}

object C {
    @TAStatic const val z = 1 // ERROR '@JvmStatic' annotation is useless for const or '@JvmField' properties
    @TAStatic @JvmField val x = 1 // ERROR '@JvmStatic' annotation is useless for const or '@JvmField' properties
    const val ok = 1
}
