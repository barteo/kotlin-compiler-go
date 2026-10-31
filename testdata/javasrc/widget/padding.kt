package widget

var Widget.verticalPadding: Int
    get() = paddingTop
    set(value) = setPadding(0, value, 0, value)
