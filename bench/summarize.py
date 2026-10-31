"""Prints a markdown table from a hyperfine --export-json file.

    python3 bench/summarize.py results.json label...

hyperfine's JSON keeps the expanded command, not the -n name, so the
labels are passed in the same order as the commands.
"""
import json
import sys

results = json.load(open(sys.argv[1]))["results"]
labels = sys.argv[2:]
base = results[0]["summary"]["time_wall_clock"]["mean"]
print("| Command | Wall time | vs `kc -j 1` | CPU time (user+sys) | Peak memory |")
print("|:---|---:|---:|---:|---:|")
for i, r in enumerate(results):
    name = labels[i] if i < len(labels) else r["command"]
    s = r["summary"]
    wall = s["time_wall_clock"]
    cpu = s["time_user"]["mean"] + s["time_system"]["mean"]
    mem = s["memory_peak_resident"]["mean"] / 2**20
    ratio = wall["mean"] / base
    rel = f"{ratio:.1f}x slower" if ratio >= 1.05 else (f"{1 / ratio:.1f}x faster" if ratio <= 0.95 else "—")
    # hyperfine has no standard deviation for a single run.
    spread = f" ± {wall['stddev'] * 1000:,.0f}" if wall["stddev"] is not None else ""
    print(f"| `{name}` | {wall['mean'] * 1000:,.0f}{spread} ms | {rel} "
          f"| {cpu * 1000:,.0f} ms | {mem:,.0f} MB |")
