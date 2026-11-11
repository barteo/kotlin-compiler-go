// A library's Java class with package-private members, which its friends'
// code of the same package uses, as a module's unit tests do.
package cmd

fun describe(c: Command) = "command ${c.name()}"
