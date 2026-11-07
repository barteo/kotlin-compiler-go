package util;

public class Flags {
    static final String KIND = "flags";

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

    public boolean isReady() {
        return ready;
    }

    public void setReady(boolean r) {
        ready = r;
    }
}
