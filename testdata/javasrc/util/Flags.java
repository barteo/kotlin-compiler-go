package util;

public class Flags {
    static final String KIND = "flags";
    protected final String tag = "tagged";

    static String describe(Flags f) {
        return KIND + " " + f.isReady();
    }

    private Boolean event = null;
    private boolean ready = true;

    public Boolean getIsEvent() {
        return event == null ? Boolean.FALSE : event;
    }

    public void setIsEvent(Boolean e) {
        event = e;
    }

    protected String tagged() {
        return tag + " " + ready;
    }

    public boolean isReady() {
        return ready;
    }

    public void setReady(boolean r) {
        ready = r;
    }
}
