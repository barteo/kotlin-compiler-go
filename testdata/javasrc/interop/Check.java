package interop;

import androidx.annotation.NonNull;

public interface Check<T> {
    boolean test(@NonNull T t);
}
