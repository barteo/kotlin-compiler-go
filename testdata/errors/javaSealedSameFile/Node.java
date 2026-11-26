public sealed interface Node {
    record Leaf(int value) implements Node {}
    record Pair(Node left, Node right) implements Node {}
}
