package app

import text.words
import text.tag
import text.firstWord as head
import units.*
import units.tag as unitsTag

fun String.local() = "app:" + this

fun main() {
    println("one two  three".words())
    println("x".tag()) // the explicit import comes before the star import
    println(5.tag())
    println("y".unitsTag())
    println(1500.m().km())
    println("z".local())
    println("first second".head)
}
