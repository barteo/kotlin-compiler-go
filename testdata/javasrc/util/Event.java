package util;

class Event {
    static final String SYNC = "sync";
    static int created = 0;
    final String name;

    Event(String name) {
        this.name = name;
        created++;
    }

    Event with(String part) {
        return new Event(name + "+" + part);
    }

    String describe() {
        return name + " (" + created + ")";
    }
}
