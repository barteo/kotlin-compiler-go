"""Prints kotlinc's per-phase times from a warm harness run.

    python3 bench/phases.py <dir>

<dir> holds the -Xdump-perf=<dir>/{run}.json files that WarmKotlinc
writes, one per run. Like the harness, it reports the median of the second
half of the runs, after the JIT has warmed up.
"""
import glob
import json
import os
import statistics
import sys

paths = sorted(glob.glob(os.path.join(sys.argv[1], "*.json")),
               key=lambda p: int(os.path.basename(p)[:-5]))
runs = [json.load(open(p)) for p in paths[len(paths) // 2:]]
phases = [
    ("Init", "initStats"),
    ("Analyze (parse, resolve, check)", "analysisStats"),
    ("FIR to IR", "translationToIrStats"),
    ("IR lowering", "irLoweringStats"),
    ("Bytecode generation", "backendStats"),
]
times = {name: statistics.median(r[key]["nanos"] for r in runs) / 1e6 for name, key in phases}
total = sum(times.values())
print(f"Median of runs {len(paths) - len(runs) + 1}-{len(paths)}, "
      f"{runs[0]['filesCount']} files, {runs[0]['linesCount']} lines.")
print()
print("| kotlinc phase | Wall time | Share |")
print("|:---|---:|---:|")
for name, _ in phases:
    print(f"| {name} | {times[name]:,.0f} ms | {times[name] * 100 / total:.0f}% |")
print(f"| **Total** | **{total:,.0f} ms** | |")
gc = statistics.median(sum(g["millis"] for g in r["gcStats"]) for r in runs)
classes = statistics.median(r["findKotlinClassStats"]["time"]["nanos"] for r in runs) / 1e6
count = runs[0]["findKotlinClassStats"]["count"]
print()
print(f"- GC pauses: {gc:,.0f} ms per run")
print(f"- Classpath lookups of Kotlin classes: {count}, {classes:,.0f} ms per run")
