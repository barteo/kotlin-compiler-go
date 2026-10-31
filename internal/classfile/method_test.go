package classfile_test

import (
	"archive/zip"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// TestDecodeStdlib reads and decodes every method of the Kotlin standard
// library. Instructions must cover the code exactly, branch targets must
// be instructions, and frames must be at instructions.
func TestDecodeStdlib(t *testing.T) {
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	z, err := zip.OpenReader(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	classes, methods, insns, unsupported, parts := 0, 0, 0, 0, 0
	for _, f := range z.File {
		if !strings.HasSuffix(f.Name, ".class") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		info, err := classfile.Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		classes++
		for _, mi := range info.Methods {
			if mi.Access&(classfile.AccAbstract|0x0100 /* native */) != 0 {
				continue
			}
			m, err := classfile.ReadMethod(data, mi.Name, mi.Desc)
			if err != nil {
				t.Fatalf("%s.%s%s: %v", info.Name, mi.Name, mi.Desc, err)
			}
			if info.Kotlin != nil && info.Kotlin.K == 5 {
				parts++
				if m.Facade != info.Kotlin.XS || m.Facade == "" {
					t.Errorf("%s: facade %q, want %q", info.Name, m.Facade, info.Kotlin.XS)
				}
			}
			methods++
			is, err := m.Decode()
			if _, ok := errors.AsType[*classfile.UnsupportedError](err); ok {
				unsupported++
				continue
			}
			if err != nil {
				t.Fatalf("%s.%s%s: %v", info.Name, mi.Name, mi.Desc, err)
			}
			insns += len(is)
			at := map[int]bool{}
			end := 0
			for _, in := range is {
				if in.Off != end {
					t.Fatalf("%s.%s%s: instruction at %d, want %d", info.Name, mi.Name, mi.Desc, in.Off, end)
				}
				at[in.Off] = true
				end += in.Len
			}
			if end != len(m.Code) {
				t.Fatalf("%s.%s%s: instructions end at %d, code at %d", info.Name, mi.Name, mi.Desc, end, len(m.Code))
			}
			for _, in := range is {
				targets := in.Targets
				if in.Target != 0 || in.Op == classfile.GOTO {
					targets = append(targets, in.Target)
				}
				for _, tg := range targets {
					if !at[tg] {
						t.Fatalf("%s.%s%s: branch at %d to %d, not an instruction", info.Name, mi.Name, mi.Desc, in.Off, tg)
					}
					if m.Frames[tg] == nil {
						t.Fatalf("%s.%s%s: branch at %d to %d, which has no frame", info.Name, mi.Name, mi.Desc, in.Off, tg)
					}
				}
			}
			for off, fr := range m.Frames {
				if !at[off] {
					t.Fatalf("%s.%s%s: frame at %d, not an instruction", info.Name, mi.Name, mi.Desc, off)
				}
				if len(fr.Locals) > m.MaxLocals {
					t.Fatalf("%s.%s%s: frame at %d has %d locals, max %d", info.Name, mi.Name, mi.Desc, off, len(fr.Locals), m.MaxLocals)
				}
			}
			for _, h := range m.Handlers {
				if m.Frames[h.Handler] == nil {
					t.Fatalf("%s.%s%s: handler at %d has no frame", info.Name, mi.Name, mi.Desc, h.Handler)
				}
			}
		}
	}
	t.Logf("%d classes, %d methods (%d in multifile parts), %d instructions; %d methods use jsr, invokedynamic or dynamic constants",
		classes, methods, parts, insns, unsupported)
	if classes < 500 || methods < 10000 {
		t.Errorf("only %d classes and %d methods", classes, methods)
	}
}

// TestReadMethod checks the code, handlers and frames of use(), which
// closes a resource in a finally block.
func TestReadMethod(t *testing.T) {
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open("", []string{jar})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	data, err := cp.Bytes("kotlin/text/StringsKt__StringsKt")
	if err != nil {
		t.Fatal(err)
	}
	m, err := classfile.ReadMethod(data, "isNotEmpty", "(Ljava/lang/CharSequence;)Z")
	if err != nil {
		t.Fatal(err)
	}
	if m.Facade != "kotlin/text/StringsKt" || m.Access&classfile.AccPrivate == 0 {
		t.Errorf("facade %q, access %#x", m.Facade, m.Access)
	}
	is, err := m.Decode()
	if err != nil {
		t.Fatal(err)
	}
	var ops []string
	for _, in := range is {
		switch in.Op {
		case classfile.ALOAD:
			ops = append(ops, "aload")
		case classfile.LDC:
			ops = append(ops, "ldc "+in.Str)
		case classfile.INVOKESTATIC, classfile.INVOKEINTERFACE:
			ops = append(ops, in.Owner+"."+in.Name)
		case classfile.IFLE, classfile.GOTO:
			ops = append(ops, "jump")
		case classfile.IRETURN:
			ops = append(ops, "ireturn")
		default:
			ops = append(ops, "const")
		}
	}
	want := "aload,ldc <this>,kotlin/jvm/internal/Intrinsics.checkNotNullParameter,aload,java/lang/CharSequence.length,jump,const,jump,const,ireturn"
	if got := strings.Join(ops, ","); got != want {
		t.Errorf("instructions:\n%s\nwant:\n%s", got, want)
	}
	if len(m.Frames) != 2 || m.Frames[20] == nil || len(m.Frames[20].Stack) != 1 || m.Frames[20].Stack[0].Tag() != classfile.VInt {
		t.Errorf("frames %v", m.Frames)
	}

	data, err = cp.Bytes("kotlin/io/CloseableKt")
	if err != nil {
		t.Fatal(err)
	}
	m, err = classfile.ReadMethod(data, "use", "(Ljava/io/Closeable;Lkotlin/jvm/functions/Function1;)Ljava/lang/Object;")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Handlers) != 4 || m.Handlers[0].Catch != "java/lang/Throwable" || m.Handlers[1].Catch != "" {
		t.Errorf("handlers %+v", m.Handlers)
	}
	h := m.Frames[m.Handlers[0].Handler]
	if h == nil || len(h.Stack) != 1 || h.Stack[0].Class() != "java/lang/Throwable" || len(h.Locals) != 3 || h.Locals[2].Tag() != classfile.VNull {
		t.Errorf("handler frame %+v", h)
	}
}
