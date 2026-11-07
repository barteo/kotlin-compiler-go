package p;

public class Counter {
    private Integer count;
    private Double scale;
    public Integer limit;
    private String label;

    public static String parse(String s) { return s == null ? null : s.trim(); }
    public String getLabel() { return label; }
    public void setLabel(String v) { label = v; }

    public Integer getCount() { return count; }
    public void setCount(Integer v) { count = v; }
    public Double getScale() { return scale; }
    public void setScale(Double v) { scale = v; }
}
