import base.Base

class Sub : Base() {
    companion object {
        fun newBundle(data: String, id: String) = newBundle(data).apply { put("id", id) }
    }
}

fun main() {
    println(Sub.newBundle("d", "i"))
}
