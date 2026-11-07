package util

// A package-private Java class, which code in its package uses.
class Events {
    private fun create(): Event = Event(Event.SYNC).with("a")

    fun describe(): String = create().describe() + " " + create().name + " " + Flags.describe(Flags())
}
