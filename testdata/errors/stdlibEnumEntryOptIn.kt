// An enum entry calls the enum's constructor, which needs opt-in when it
// is marked, with or without arguments, as kotlinc's OPT_IN_USAGE_ERROR
// with Kotlin 2.4's CheckOptInOnPureEnumEntries. The entry's annotations
// opt in, for an entry with a body too.
@RequiresOptIn
annotation class O

enum class E1 @O constructor() {
    A, // ERROR this declaration needs opt-in
    B(), // ERROR this declaration needs opt-in
    C { }, // ERROR this declaration needs opt-in
    @OptIn(O::class) D,
    @OptIn(O::class) F { };
}

enum class E2 @O constructor(val x: Int = 0) {
    A(1); // ERROR this declaration needs opt-in

    @OptIn(O::class)
    constructor(s: String) : this(s.length)
}

enum class E3 {
    A(""); // the secondary constructor, which opts in

    @OptIn(O::class)
    constructor(s: String)
}
