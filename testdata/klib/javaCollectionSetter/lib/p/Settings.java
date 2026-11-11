package p;
import java.util.*;
public class Settings {
    private final List<Long> ids = new ArrayList<>();
    private Long parent;
    public List<Long> getTaskIds() { return ids; }
    public void setTaskIds(Collection<Long> v) { ids.clear(); ids.addAll(v); }
    public Long getParentId() { return parent; }
    public void setParentId(Long v) { parent = v; }
}
