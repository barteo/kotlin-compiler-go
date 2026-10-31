package app

import model.Color
import model.Color.RED
import model.Color.GREEN as Verde
import model.Shape.Kind
import model.Shape.Kind.SQUARE
import model.Shape.Companion.UNIT
import model.Shape.Companion.DEFAULT
import model.Shape.Companion.of
import model.Registry.register
import model.Registry.count
import model.Registry.names
import model.Size
import java.lang.Integer.MAX_VALUE
import java.lang.Integer.toHexString
import java.lang.Thread.State
import java.util.concurrent.TimeUnit.SECONDS

// The explicit import comes before the package's own declarations.
val names = "app"

fun describe(c: Color) = when (c) {
    RED -> "red"
    Verde -> "green"
    Color.BLUE -> "blue"
}

fun kind(k: Kind) = when (k) {
    SQUARE -> "square"
    Kind.ROUND -> "round"
}

fun main() {
    println(describe(RED) + " " + describe(Verde) + " " + describe(Color.BLUE))
    println(kind(SQUARE) + " " + kind(Kind.ROUND))
    println(RED)
    println(Verde.ordinal)
    println(UNIT)
    println(DEFAULT.name)
    println(of("box").name)
    println(register(2))
    println(register(3))
    count = 10
    count += 1
    count++
    println(count)
    println(names)
    println(MAX_VALUE)
    println(toHexString(255))
    println(State.NEW)
    println(SECONDS.toMillis(2))
    println(Size.SMALL.kind)
    println(Size.LARGE.kind)
}
