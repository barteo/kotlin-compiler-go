package android.os;

// Android's Binder, for the parcelize tests: the fake Parcel passes
// binders by reference, so a binder reads back as itself.
public class Binder implements IBinder {
    private final String name;

    public Binder(String name) {
        this.name = name;
    }

    @Override
    public String toString() {
        return "Binder(" + name + ")";
    }
}
