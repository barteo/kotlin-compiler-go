// Two files' private extensions of the same name don't clash.

private fun String.preview(): String = this + "!"

fun previewA() = "a".preview()
