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
        if edits:
            self.change(apply_edits(self.text, edits), notify)


def apply_edits(source, edits):
    text = source
    for edit in sorted(edits, key=lambda e: offset(source, e["range"]["start"]), reverse=True):
        a, b = (offset(source, edit["range"][k]) for k in ("start", "end"))
        text = text[:a] + edit["newText"] + text[b:]
    return text


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
    formatting = {}
    for name, case in json.loads((fixture / "formatting.json").read_text()).items():
        buffer.change(case["text"], notify)
        params = {"textDocument": doc, "options": case["options"]}
        method = "formatting"
        if "range" in case:
            method = "rangeFormatting"
            params["range"] = case["range"]
        edits = request("textDocument/" + method, params)
        if not isinstance(edits, list):
            raise RuntimeError(f"{name}: expected formatting edits, received {edits}")
        buffer.apply({"changes": {buffer.uri: edits}}, notify)
        formatting[name] = buffer.text
    out["formatting"] = formatting
    return out


def record_navigation(request, notify, buffer, fixture, settings):
    doc = {"uri": buffer.uri}
    defs = {}
    for name, marked in json.loads((fixture / "cases.json").read_text()).items():
        prefix, suffix = marked.split("|")
        buffer.change(prefix + suffix + "\n", notify)
        position = {"line": prefix.count("\n"), "character": len(prefix.split("\n")[-1].encode("utf-16-le")) // 2}
        deadline = time.monotonic() + 15
        while True:
            links = request("textDocument/definition", {"textDocument": doc, "position": position})
            if isinstance(links, list) and len(links) == 1:
                break
            if time.monotonic() >= deadline:
                raise RuntimeError(f"{name}: {links}")
            time.sleep(0.3)
        link = links[0]
        link["targetUri"] = link["targetUri"].rsplit("/", 1)[-1]
        defs[name] = link
    source = (fixture / "Hints.kt").read_text()
    hints = {}
    for name, options in (("default", {}), ("chains", {"call.chains": True}), ("partialEdit", {}),
                          ("disabled", {"lambda.return": False, "lambda.receivers.parameters": False})):
        if name == "partialEdit":
            buffer.change(source.replace("    42\n", "    /* 😀 */ 420\n"), notify)
        else:
            buffer.change(source, notify)
        settings.clear()
        # The release's provider reads hints.lambda.return from the
        # jetbrains.kotlin section. Its extension lists settings.lambda.return.
        defaults = {"type.function.parameter": True, "lambda.return": True,
                    "lambda.receivers.parameters": True, "call.chains": False}
        settings["jetbrains.kotlin"] = {"hints": {**defaults, **options}}
        notify("workspace/didChangeConfiguration", {"settings": settings.copy()})
        r = {"start": {"line": 12, "character": 0}, "end": {"line": source.count("\n"), "character": 0}}
        if name == "partialEdit":
            line = next(i for i, text in enumerate(buffer.text.splitlines()) if "420" in text)
            r = {"start": {"line": line - 2, "character": 0}, "end": {"line": line + 1, "character": 0}}
        values = request("textDocument/inlayHint", {"textDocument": doc, "range": r})
        if not isinstance(values, list):
            raise RuntimeError(f"{name}: {values}")
        hints[name] = []
        for value in values:
            h = {k: v for k, v in value.items() if k in ("position", "label", "kind", "paddingLeft", "paddingRight")}
            if isinstance(h["label"], list):
                h["label"] = "".join(part["value"] for part in h["label"])
            hints[name].append(h)
        hints[name].sort(key=lambda h: (h["position"]["line"], h["position"]["character"]))
    if not any(h["label"].startswith("^") for h in hints["default"]):
        raise RuntimeError("Reference returned no lambda result hints")
    if not any(h.get("paddingLeft") for h in hints["chains"]):
        raise RuntimeError("Reference returned no intermediate chain types")
    return {"definitions": defs, "hints": hints}


def record_intentions(request, notify, buffer, fixture, settings):
    doc = {"uri": buffer.uri}
    buffer.change((fixture / "Outline.kt").read_text(), notify)
    outline = request("textDocument/documentSymbol", {"textDocument": doc})
    settings["jetbrains.kotlin"] = {"hints": {"type.function.return": True}}
    notify("workspace/didChangeConfiguration", {"settings": settings})
    buffer.change("package sample\nimport library.packet\nfun result() = packet()\n", notify)
    hints = request("textDocument/inlayHint", {"textDocument": doc, "range": {
        "start": {"line": 0, "character": 0}, "end": {"line": 3, "character": 0}}})
    if not hints:
        raise RuntimeError("Expected a type hint for result()")
    resolved = []
    for hint in hints:
        item = request("inlayHint/resolve", hint)
        for part in item["label"]:
            if "location" in part:
                loc = part["location"]
                loc["uri"] = loc["uri"].rsplit("/", 1)[-1]
        resolved.append({k: v for k, v in item.items() if k in ("label", "position", "kind", "paddingLeft", "paddingRight")})
    if not any(p.get("location", {}).get("uri") == "Library.kt" for h in resolved for p in h["label"]):
        raise RuntimeError(f"Missing cross-module type link: {resolved}")
    results = {}
    for case, marked in json.loads((fixture / "cases.json").read_text()).items():
        prefix, suffix = marked.split("|")
        source = prefix + suffix
        buffer.change(source, notify)
        at = {"line": prefix.count("\n"), "character": len(prefix.rsplit("\n", 1)[-1])}
        actions = request("textDocument/codeAction", {"textDocument": doc, "range": {"start": at, "end": at},
                          "context": {"diagnostics": []}}) or []
        target = {"property": "Specify type explicitly", "local": "Specify type explicitly",
                  "function": "Specify return type explicitly", "collision": "Specify return type explicitly"}
        title = target.get(case, "Add names to call arguments")
        action = next((a for a in actions if a["title"] == title), None)
        if case in ("java", "alreadyNamed"):
            if action:
                raise RuntimeError(f"Unexpected action for {case}: {action}")
            results[case] = {"absent": title}
            continue
        if not action:
            raise RuntimeError(f"{case}: {actions}")
        if "edit" in action:
            buffer.apply(action["edit"], notify)
        if "command" in action:
            request("workspace/executeCommand", action["command"])
        if buffer.text == source:
            raise RuntimeError(f"No edit for {case}: {action}")
        results[case] = {"title": action["title"], "kind": action["kind"], "text": buffer.text}
    return {"outline": outline, "hints": resolved, "actions": results}
