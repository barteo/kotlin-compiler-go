package widget;

// A subclass whose static method hides its superclass's.
public class Fancy extends Widget {
    public static Fancy getInstance() { return new Fancy(); }

    public String fancy() { return "fancy"; }
}
