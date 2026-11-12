package j;
public class EditText extends TextView {
    @Override public Editable getText() { return new Editable(text == null ? "" : text.toString()); }
}
