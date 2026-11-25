"""Exercise the same editing buffer against the imported reference project."""
import json
import time


def offset(text, pos):
    lines = text.splitlines(keepends=True)
    line, column = pos["line"], pos["character"]
    prefix = "".join(lines[:line])
    tail = "".join(lines[line:]).encode("utf-16-le")[:column * 2]
    return len(prefix) + len(tail.decode("utf-16-le"))


class Buffer:
    def __init__(self, uri):
        self.uri, self.text, self.version = uri, "", 0

    def change(self, text, notify):
        self.text = text
        self.version += 1
        doc = {"uri": self.uri, "version": self.version}
        if self.version == 1:
            notify("textDocument/didOpen", {"textDocument": dict(doc, text=text, languageId="kotlin")})
        else:
            notify("textDocument/didChange", {"textDocument": doc, "contentChanges": [{"text": text}]})

    def apply(self, edit, notify):
        edits = list(edit.get("changes", {}).get(self.uri, []))
        for change in edit.get("documentChanges", []):
            assert change["textDocument"]["uri"] == self.uri
            edits.extend(change["edits"])
        text = self.text
        for e in sorted(edits, key=lambda e: offset(text, e["range"]["start"]), reverse=True):
            a, b = (offset(self.text, e["range"][k]) for k in ("start", "end"))
            text = text[:a] + e["newText"] + text[b:]
        if edits:
            self.change(text, notify)


def labels(result, legend):
    data = result["data"]
    matches = []
    line = column = 0
    for i in range(0, len(data), 5):
        delta, start, length, kind, bits = data[i:i + 5]
        line += delta
        column = start if delta else column + start
        if legend["tokenTypes"][kind] == "label":
            modifiers = [m for j, m in enumerate(legend["tokenModifiers"]) if bits & (1 << j)]
            matches.append({"line": line, "character": column, "length": length, "modifiers": modifiers})
    return matches


def record_cases(request, notify, buffer, fixture, legend):
    doc = {"uri": buffer.uri}
    out = {}
    for case, marked in json.loads((fixture / "cases.json").read_text()).items():
        prefix, suffix = marked.split("|")
        pos = {"line": prefix.count("\n"), "character": len(prefix.rsplit("\n", 1)[-1].encode("utf-16-le")) // 2}
        params = {"textDocument": doc, "position": pos}
        buffer.change(prefix + suffix, notify)
        if case == "completion":
            result = request("textDocument/completion", params)
            items = result["items"] if isinstance(result, dict) else result
            names = [i["label"] for i in items if i["label"] in ("editingLocal", "editingMember", "editingGlobal")]
            selected = next(i for i in items if i["label"] == "editingLocal")
            e = selected.get("textEdit")
            if e:
                buffer.apply({"changes": {buffer.uri: [{"range": e.get("replace", e.get("range")), "newText": e["newText"]}]}}, notify)
            if "command" in selected:
                request("workspace/executeCommand", selected["command"])
            if "editingLocal\n" not in buffer.text:
                raise RuntimeError(f"completion did not apply: {selected} {buffer.text}")
            out[case] = {"order": names, "text": buffer.text}
        elif case == "definition":
            links = request("textDocument/definition", params)
            if not links or "targetUri" not in links[0]:
                raise RuntimeError(f"Expected a definition link: {links}")
            out[case] = links
        elif case == "labels":
            full = labels(request("textDocument/semanticTokens/full", {"textDocument": doc}), legend)
            # Select just the labeled lambda line.
            r = {"start": pos, "end": {"line": pos["line"] + 1, "character": 0}}
            limited = labels(request("textDocument/semanticTokens/range", {"textDocument": doc, "range": r}), legend)
            if len(full) != 5 or len(limited) != 2:
                raise RuntimeError(f"Missing label tokens: {full}, {limited}")
            out[case] = {"full": full, "range": limited}
        elif case == "signature":
            deadline = time.monotonic() + 15
            while True:
                signatures = request("textDocument/signatureHelp", params) or {}
                if signatures.get("signatures"):
                    out[case] = signatures
                    break
                if time.monotonic() > deadline:
                    raise RuntimeError(f"Reference returned no signature for {marked}")
                time.sleep(0.3)
        else:
            deadline = time.monotonic() + 25
            while True:
                ds = request("textDocument/diagnostic", {"textDocument": doc})["items"]
                actions = request("textDocument/codeAction", {"textDocument": doc, "range": {"start": pos, "end": pos},
                    "context": {"diagnostics": ds, "only": ["quickfix"]}}) or []
                if case == "unknownArgument":
                    diag = next((d for d in ds if d["code"] == "NAMED_PARAMETER_NOT_FOUND" and
                                 d["range"]["start"] == pos), None)
                    if diag:
                        out[case] = {"diagnostics": [{"range": diag["range"], "severity": diag["severity"]}],
                                     "titles": [a["title"] for a in actions if "remove" in a["title"].lower()
                                                and "argument" in a["title"].lower()]}
                        break
                    if time.monotonic() >= deadline:
                        raise RuntimeError(f"Missing named argument diagnostic: {ds}")
                    time.sleep(0.3)
                    continue
                words = {"assertion": ("remove", "!!"), "safeCall": ("replace",), "elvis": ("elvis",),
                         "soleArgument": ("remove", "argument"), "extraArgument": ("remove", "argument"),
                         "trailingArgument": ("remove", "argument"), "inner": ("remove", "inner"), "import": ("import", "editingpacket")}[case]
                action = next((a for a in actions if all(w in a["title"].lower() for w in words)), None)
                if action: break
                if time.monotonic() >= deadline: raise RuntimeError(f"{case}: {ds} {actions}")
                time.sleep(0.3)
            if case == "unknownArgument":
                continue
            if "edit" in action: buffer.apply(action["edit"], notify)
            if "command" in action: request("workspace/executeCommand", action["command"])
            if buffer.text == prefix + suffix: raise RuntimeError(f"No edit for {case}: {action}")
            # Compare the diagnostic at the requested token. Other inspections
            # (e.g. inferred error return type) have separate triggers.
            def at_cursor(d):
                key = lambda p: (p["line"], p["character"])
                return key(d["range"]["start"]) <= key(pos) < key(d["range"]["end"])

            relevant = [d for d in ds if at_cursor(d)]
            if len(relevant) != 1:
                raise RuntimeError(f"Ambiguous diagnostic for {case}: {ds}")
            deadline = time.monotonic() + 15
            while True:
                after = request("textDocument/diagnostic", {"textDocument": doc})["items"]
                if not after:
                    break
                if time.monotonic() >= deadline:
                    raise RuntimeError(f"Outstanding diagnostics after {case}: {after}")
                time.sleep(0.3)
            d = relevant[0]
            out[case] = {"diagnostics": [{"range": d["range"], "severity": d["severity"]}],
                         "text": buffer.text, "after": after}
            if case.endswith("Argument"):
                out[case]["title"] = action["title"]
    return out
