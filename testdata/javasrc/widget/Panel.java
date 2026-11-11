package widget;

import java.util.List;

public class Panel {
    private final text.Panel config;
    private final int scale;

    public Panel(text.Panel config, int scale) {
        this.config = config;
        this.scale = scale;
    }

    public List<String> getRows() {
        return config.getRows();
    }

    public int getScale() {
        return scale;
    }
}
