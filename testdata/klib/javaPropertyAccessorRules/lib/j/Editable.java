package j;
public class Editable implements CharSequence {
    private final String s;
    public Editable(String s) { this.s = s; }
    public int length() { return s.length(); }
    public char charAt(int i) { return s.charAt(i); }
    public CharSequence subSequence(int a, int b) { return s.subSequence(a, b); }
    public String toString() { return s; }
}
