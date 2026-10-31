package widget;

import java.util.List;

// A Java class whose accessors Kotlin sees as properties, as Android's
// View, with a listener interface and a static factory.
public class Widget {
    public interface OnDismiss {
        void onDismiss(Widget w);
    }

    public interface Filter {
        CharSequence filter(CharSequence s);
    }

    private boolean enabled = true;
    private int lineHeight;
    private int top;
    private Filter[] filters = new Filter[0];
    private List<String> items;
    private OnDismiss listener;

    public boolean isEnabled() { return enabled; }
    public void setEnabled(boolean enabled) { this.enabled = enabled; }
    public int getLineHeight() { return lineHeight; }
    public void setLineHeight(int h) { lineHeight = h; }
    public int getPaddingTop() { return top; }
    public void setPadding(int left, int top, int right, int bottom) { this.top = top; }
    public Filter[] getFilters() { return filters; }
    public void setFilters(Filter[] filters) { this.filters = filters; }
    public List<String> getItems() { return items; }
    public void setItems(List<String> items) { this.items = items; }
    public void setOnDismissListener(OnDismiss l) { listener = l; }

    public String dismiss() {
        if (listener == null) return "no listener";
        listener.onDismiss(this);
        return "dismissed";
    }

    public static Widget getInstance() { return new Widget(); }
}
