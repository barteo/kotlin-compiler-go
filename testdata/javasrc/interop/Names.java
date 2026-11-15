package interop;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

// Collections with nullability annotations, as an annotated library's.
public class Names {
    public interface Listener {
        void changed(@NonNull List<String> names);
    }

    public interface Provider {
        @NonNull
        List<String> names();

        @Nullable
        List<String> more();
    }

    private final List<String> names;

    public Names(int skip, @NonNull List<String> names) {
        this.names = new ArrayList<String>(names.subList(skip, names.size()));
    }

    public static int count(@NonNull List<String> names, @Nullable Map<String, String> more) {
        return names.size() + (more == null ? 0 : more.size());
    }

    public static int provided(@NonNull Provider provider) {
        List<String> more = provider.more();
        return provider.names().size() + (more == null ? 0 : more.size());
    }

    @NonNull
    public List<String> all() {
        return names;
    }

    public void notify(@NonNull Listener listener) {
        listener.changed(names);
    }
}
