import java.lang.reflect.Modifier

class Model {
    var priority = ""

    companion object {
        fun from(level: Int): Model {
            val model = Model()
            model.priority = parse(level)
            return model
        }

        // The outer class's static method is private too, and calls the
        // companion's through its accessor.
        @JvmStatic
        private fun parse(level: Int): String = "level $level"
    }
}

fun main() {
    println(Model.from(3).priority)
    val method = Model::class.java.getDeclaredMethod("parse", Int::class.javaPrimitiveType)
    println(Modifier.toString(method.modifiers))
    method.isAccessible = true
    println(method.invoke(null, 4))
}
