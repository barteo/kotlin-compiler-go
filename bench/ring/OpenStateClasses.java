// Removes ACC_FINAL from the JMH @State classes in a directory of class
// files, and from their methods, as the all-open compiler plugin does for
// kotlinx-benchmark's Gradle build: JMH subclasses the state classes.
// bench/ring.sh runs it on both compilers' output, so they're treated alike.
//
//   java -cp asm.jar OpenStateClasses.java <class dir>

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.stream.Stream;

import org.objectweb.asm.AnnotationVisitor;
import org.objectweb.asm.ClassReader;
import org.objectweb.asm.ClassVisitor;
import org.objectweb.asm.ClassWriter;
import org.objectweb.asm.MethodVisitor;
import org.objectweb.asm.Opcodes;

public class OpenStateClasses {
    static final String STATE = "Lorg/openjdk/jmh/annotations/State;";

    public static void main(String[] args) throws IOException {
        List<Path> files;
        try (Stream<Path> s = Files.walk(Path.of(args[0]))) {
            files = s.filter(p -> p.toString().endsWith(".class")).toList();
        }
        int opened = 0;
        for (Path p : files) {
            byte[] in = Files.readAllBytes(p);
            if (!isState(new ClassReader(in))) {
                continue;
            }
            ClassReader r = new ClassReader(in);
            ClassWriter w = new ClassWriter(r, 0);
            r.accept(new ClassVisitor(Opcodes.ASM9, w) {
                @Override
                public void visit(int version, int access, String name, String sig, String sup, String[] ifaces) {
                    super.visit(version, access & ~Opcodes.ACC_FINAL, name, sig, sup, ifaces);
                }

                @Override
                public void visitInnerClass(String name, String outer, String inner, int access) {
                    // The class's own InnerClasses entry repeats its flags.
                    if (name.equals(r.getClassName())) {
                        access &= ~Opcodes.ACC_FINAL;
                    }
                    super.visitInnerClass(name, outer, inner, access);
                }

                @Override
                public MethodVisitor visitMethod(int access, String name, String desc, String sig, String[] exc) {
                    if ((access & (Opcodes.ACC_PRIVATE | Opcodes.ACC_STATIC)) == 0) {
                        access &= ~Opcodes.ACC_FINAL;
                    }
                    return super.visitMethod(access, name, desc, sig, exc);
                }
            }, 0);
            Files.write(p, w.toByteArray());
            opened++;
        }
        System.out.println("opened " + opened + " @State classes in " + args[0]);
    }

    static boolean isState(ClassReader r) {
        boolean[] found = {false};
        r.accept(new ClassVisitor(Opcodes.ASM9) {
            @Override
            public AnnotationVisitor visitAnnotation(String desc, boolean visible) {
                if (desc.equals(STATE)) {
                    found[0] = true;
                }
                return null;
            }
        }, ClassReader.SKIP_CODE | ClassReader.SKIP_DEBUG | ClassReader.SKIP_FRAMES);
        return found[0];
    }
}
