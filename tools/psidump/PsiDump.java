// PsiDump prints the PSI tree that stock Kotlin 2.4.21 builds for each
// input file, as golden data for the psi package's tree-shape tests.
//
// It is a reference tool, run by hand; CI never needs a JVM. See
// tools/psidump/README.md for how to fetch the pinned jars and run it.
//
// Output: one line per node in preorder,
//
//	<depth> <KIND> <start> <end>
//
// where KIND is a name from psi/kinds.txt (or ?debugName for an element
// type psi has no kind for) and start/end are UTF-16 offsets, as in the
// IntelliJ API. The root is always FILE.

import java.lang.reflect.Field;
import java.lang.reflect.Modifier;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

import org.jetbrains.kotlin.KtNodeTypes;
import org.jetbrains.kotlin.cli.FrontendConfigurationKeysKt;
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles;
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment;
import org.jetbrains.kotlin.com.intellij.lang.ASTNode;
import org.jetbrains.kotlin.com.intellij.openapi.Disposable;
import org.jetbrains.kotlin.com.intellij.openapi.util.Disposer;
import org.jetbrains.kotlin.com.intellij.psi.TokenType;
import org.jetbrains.kotlin.com.intellij.psi.tree.IElementType;
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar;
import org.jetbrains.kotlin.config.CompilerConfiguration;
import org.jetbrains.kotlin.kdoc.lexer.KDocTokens;
import org.jetbrains.kotlin.lexer.KtTokens;
import org.jetbrains.kotlin.psi.KtFile;
import org.jetbrains.kotlin.psi.KtPsiFactory;
import org.jetbrains.kotlin.psi.stubs.elements.KtStubElementTypes;

public final class PsiDump {
    private static final Map<IElementType, String> NAMES = new IdentityHashMap<>();
    // Field names that psi/kinds.txt spells differently.
    private static final Map<String, String> ALIASES = Map.of("EXCLEQEQEQ", "EXCLEQEQ");

    public static void main(String[] args) throws Exception {
        if (args.length < 2) {
            System.err.println("usage: PsiDump KINDS_FILE OUT_DIR [--text] [--rel ROOT] FILE...");
            System.exit(2);
        }
        Set<String> kinds = new HashSet<>(List.of(Files.readString(Path.of(args[0])).trim().split("\\s+")));
        Path out = Path.of(args[1]);
        boolean text = false;
        Path rel = null;
        List<Path> inputs = new ArrayList<>();
        for (int i = 2; i < args.length; i++) {
            if (args[i].equals("--text")) {
                text = true;
            } else if (args[i].equals("--rel")) {
                rel = Path.of(args[++i]).toAbsolutePath().normalize();
            } else {
                inputs.add(Path.of(args[i]));
            }
        }
        for (Class<?> c : new Class<?>[] {KtNodeTypes.class, KtStubElementTypes.class, KtTokens.class, KDocTokens.class, TokenType.class}) {
            for (Field f : c.getFields()) {
                if (!Modifier.isStatic(f.getModifiers()) || !IElementType.class.isAssignableFrom(f.getType())) {
                    continue;
                }
                IElementType t = (IElementType) f.get(null);
                if (t == null) {
                    continue;
                }
                String fname = ALIASES.getOrDefault(f.getName(), f.getName());
                String prev = NAMES.get(t);
                // Prefer a name psi has; among those, the first class wins
                // (KtNodeTypes before KtStubElementTypes).
                if (prev == null || !kinds.contains(prev) && kinds.contains(fname)) {
                    NAMES.put(t, fname);
                }
            }
        }
        Disposable root = Disposer.newDisposable();
        try {
            CompilerConfiguration conf = new CompilerConfiguration();
            FrontendConfigurationKeysKt.setExtensionsStorage(conf, new CompilerPluginRegistrar.ExtensionStorage());
            KotlinCoreEnvironment env = KotlinCoreEnvironment.createForProduction(root, conf, EnvironmentConfigFiles.JVM_CONFIG_FILES);
            KtPsiFactory factory = new KtPsiFactory(env.getProject(), false);
            for (Path in : inputs) {
                String src = Files.readString(in, StandardCharsets.UTF_8);
                KtFile file = factory.createFile(in.getFileName().toString(), src);
                StringBuilder sb = new StringBuilder();
                dump(file.getNode(), 0, sb, text, kinds);
                String name = rel != null
                    ? rel.relativize(in.toAbsolutePath().normalize()).toString()
                    : in.getFileName().toString();
                Path dst = out.resolve(name + ".tree");
                Files.createDirectories(dst.getParent());
                Files.writeString(dst, sb.toString(), StandardCharsets.UTF_8);
            }
        } finally {
            Disposer.dispose(root);
        }
    }

    private static void dump(ASTNode node, int depth, StringBuilder sb, boolean text, Set<String> kinds) {
        String name = depth == 0 ? "FILE" : NAMES.get(node.getElementType());
        if (name == null || !kinds.contains(name)) {
            name = "?" + (name != null ? name : node.getElementType().toString()).replace(' ', '_');
        }
        sb.append(depth).append(' ').append(name).append(' ')
            .append(node.getStartOffset()).append(' ').append(node.getStartOffset() + node.getTextLength());
        ASTNode child = node.getFirstChildNode();
        if (text && node.getPsi() instanceof org.jetbrains.kotlin.com.intellij.psi.PsiErrorElement err) {
            sb.append(" !").append(quote(err.getErrorDescription()));
        }
        if (text && child == null) {
            sb.append(' ').append(quote(node.getText()));
        }
        sb.append('\n');
        for (; child != null; child = child.getTreeNext()) {
            dump(child, depth + 1, sb, text, kinds);
        }
    }

    private static String quote(String s) {
        StringBuilder sb = new StringBuilder("\"");
        for (char c : s.toCharArray()) {
            switch (c) {
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                default -> sb.append(c);
            }
        }
        return sb.append('"').toString();
    }
}
