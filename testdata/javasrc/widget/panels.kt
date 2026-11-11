package widget

import text.Panel

// Panel(config, 2) is this package's Java Panel: the imported one can't
// take the arguments.
class Panels(private val config: Panel) {
    fun build(): String {
        val panel = Panel(config, 2)
        return "${panel.rows} x${panel.scale} ${config.title}"
    }
}
