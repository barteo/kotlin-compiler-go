"""Groups Java Flight Recorder samples of kotlinc by compiler phase.

    jfr print --stack-depth 2048 --events jdk.ExecutionSample rec.jfr > samples.txt
    python3 bench/profile.py samples.txt

The samples are read as text, one line at a time: with deep stacks,
jfr's JSON is tens of gigabytes, more than fits in memory.

Each sample is assigned to a phase by the frames on its stack, then the
lowering and checker samples are broken down by the lowering or checker
class that was running. Shares are of all samples, so they are
approximate; use bench/phases.py for times.
"""
import collections
import re
import sys


def frame(line):
    """Returns a frame as pkg/Class.method from jfr's text,
    pkg.Class.method(Args) line: N."""
    sig = line.strip().split("(", 1)[0]
    cls, _, method = sig.rpartition(".")
    return cls.replace(".", "/") + "." + method


def stacks(path):
    frames, in_trace = None, False
    with open(path) as f:
        for line in f:
            s = line.strip()
            if s.startswith("jdk.ExecutionSample"):
                frames = []
            elif s == "stackTrace = [":
                in_trace = True
            elif in_trace and s == "]":
                in_trace = False
            elif in_trace and s and s != "...":
                frames.append(frame(s))
            elif s == "}" and frames is not None:
                yield list(reversed(frames))  # root first
                frames = None


def has(stack, *subs):
    return any(s in f for f in stack for s in subs)


def phase(s):
    if has(s, "JvmFrontendPipelinePhase"):
        if has(s, "fir/java/", "deserialization", "JvmClassFileBasedSymbolProvider",
               "KotlinCliJavaFileManager", "load/kotlin", "ZipFile", "JavaClassFinder"):
            return "Frontend: loading classpath symbols"
        if has(s, "LightTree2Fir", "lightTree"):
            if has(s, "PsiBuilder", "/parsing/", "/lexer/", "KotlinLightParser"):
                return "Frontend: lexing and parsing"
            return "Frontend: building raw FIR"
        if has(s, "runCheckers", "fir/analysis/"):
            return "Frontend: checkers (diagnostics)"
        if has(s, "body/resolve"):
            return "Frontend: body resolution (calls, types, inference)"
        if has(s, "fir/resolve/transformers"):
            return "Frontend: declaration resolution"
        return "Frontend: other"
    if has(s, "JvmFir2IrPipelinePhase"):
        return "FIR to IR"
    if has(s, "invokeLowerings", "runLowerings"):
        return "IR lowering"
    if has(s, "invokeCodegen", "runCodegen"):
        return "Bytecode generation"
    return "Other"


def short(frame):
    return frame.rsplit(".", 1)[0].split("/")[-1].split("$")[0]


phases = collections.Counter()
lowerings = collections.Counter()
checkers = collections.Counter()
n = 0
for s in stacks(sys.argv[1]):
    n += 1
    p = phase(s)
    phases[p] += 1
    if p == "IR lowering":
        name = next((short(f) for f in s if re.search(r"Lowering(\$\w+)*\.", f)), "(pass infrastructure)")
        lowerings[name] += 1
    elif p.startswith("Frontend: checkers"):
        name = next((short(f) for f in reversed(s) if "checkers/" in f and "Checker" in f), "(tree traversal)")
        checkers[name] += 1

print(f"{n:,} samples")
print()
print("| Phase | Share of samples |")
print("|:---|---:|")
for k, v in phases.most_common():
    print(f"| {k} | {v * 100 / n:.1f}% |")
for title, c in [("lowering", lowerings), ("checker", checkers)]:
    print()
    print(f"{len(c)} distinct {title} classes sampled; the largest:")
    print()
    for k, v in c.most_common(5):
        print(f"- {k}: {v * 100 / n:.1f}%")
