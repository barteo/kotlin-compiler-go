package interop;

import androidx.annotation.NonNull;

// Wrapper types that an annotation makes non-null: Kotlin's Long and Int,
// which the members take and hold boxed.
public class Ids {
    @NonNull
    public Long last = 7L;

    public Ids(@NonNull Integer start) {
        last += start;
    }

    public static String show(@NonNull Long id, long other) {
        return id + ":" + other;
    }

    @NonNull
    public static Long next(@NonNull Long id) {
        return id + 1;
    }

    @NonNull
    public Long get() {
        return last;
    }
}
