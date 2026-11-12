package text;

// Calls of Kotlin's @JvmStatic extensions and delegated properties.
public final class StaticCalls {
    public static String all() {
        StringBuilder sb = new StringBuilder("abc");
        Strings.setLast(sb, 'z');
        Counter.setFirst(sb, 'q');
        Strings.setLevel(4);
        return Strings.shout("a", 2) + " " + Strings.getWords("x y") + " " + sb + " " + Strings.getLast(sb) + Counter.getFirst(sb)
            + " " + Strings.getGreeting() + " " + Strings.getLevel()
            + " " + Counter.times3(5) + " " + Counter.getEven(3) + " " + Counter.getStart();
    }
}
