package cmd;

public class Command {
    protected static final String RELATION = "relation";
    protected static int counter = 3;
    boolean zipped;
    String id = "c1";

    static Long parseId(String response) {
        return Long.parseLong(response.trim());
    }

    String idString() {
        return "id=" + id;
    }

    protected static String tag(String s) {
        return "<" + s + ">";
    }

    public String name() {
        return "cmd";
    }
}
