package util;

public class LookupUser {
    public static int count() {
        return new Lookup().count("a", "b");
    }
}
