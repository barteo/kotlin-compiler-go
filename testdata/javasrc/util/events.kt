package util

// Package-private Java classes and members, and protected ones, which
// code in their package uses.
class Events {
    private fun create(): Event = Event(Event.SYNC).with("a")

    fun describe(): String = create().describe() + " " + create().name + " " + Flags.describe(Flags()) + " " + Flags().tag + " " + Flags().tagged()
}
