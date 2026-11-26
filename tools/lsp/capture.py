"""Record editor operations from the pinned JetBrains Kotlin language server."""
import argparse
import json
import os
import shutil
from pathlib import Path
import subprocess
import tempfile
import time
import zipfile
from queue import Queue
from threading import Thread

import editing

FIXTURES = Path(__file__).resolve().parents[2] / "internal/lsp/testdata"
RELEASE = "271.2085.0"
GRADLE_PROJECTS = ("gradle", "editing", "navigation", "moves", "intentions")


def send(process, **message):
    body = json.dumps({"jsonrpc": "2.0", **message}).encode()
    process.stdin.write(f"Content-Length: {len(body)}\r\n\r\n".encode() + body)
    process.stdin.flush()


def read_messages(stream, messages):
    try:
        while True:
            headers = {}
            while True:
                line = stream.readline()
                if not line:
                    return
                if not line.strip():
                    break
                key, value = line.decode().split(":", 1)
                headers[key.lower()] = value.strip()
            messages.put(json.loads(stream.read(int(headers["content-length"]))))
    finally:
        messages.put(None)


def capture(process, messages, root, state, project):
    registrations = []
    request_id = 0
    buffer = editing.Buffer((root / "src/main/kotlin/Editing.kt").as_uri())
    settings = {}
    def notify(method, params):
        send(process, method=method, params=params)

    def request(method, params):
        nonlocal request_id
        request_id += 1
        send(process, id=request_id, method=method, params=params)
        while True:
            msg = messages.get(timeout=90)
            if msg is None:
                raise RuntimeError("Reference server exited")
            if "method" not in msg and msg.get("id") == request_id:
                return msg.get("result", msg)
            if "method" in msg and "id" in msg:
                result = None
                if msg["method"] == "workspace/configuration":
                    result = [settings.get(item.get("section")) for item in msg["params"]["items"]]
                if msg["method"] == "client/registerCapability":
                    registrations.extend(msg["params"]["registrations"])
                if msg["method"] == "workspace/applyEdit" and project in ("editing", "intentions"):
                    buffer.apply(msg["params"]["edit"], notify)
                    result = {"applied": True}
                send(process, id=msg["id"], result=result)

    caps = {"workspace": {"configuration": True, "workspaceFolders": True},
            "textDocument": {"documentSymbol": {"hierarchicalDocumentSymbolSupport": True},
                             "foldingRange": {}, "hover": {"contentFormat": ["markdown"]}}}
    if project in ("editing", "navigation"):
        caps["textDocument"]["definition"] = {"linkSupport": True}
    if project == "moves":
        caps["workspace"].update({"fileOperations": {"willRename": True, "didRename": True},
                                  "workspaceEdit": {"documentChanges": True, "resourceOperations": ["rename"]}})
    if project == "intentions":
        caps["textDocument"]["inlayHint"] = {"resolveSupport": {"properties": ["label.location"]}}
    if project == "navigation":
        caps["textDocument"]["inlayHint"] = {}
    if project == "editing":
        caps["textDocument"]["semanticTokens"] = {
            "requests": {"full": True, "range": True}, "formats": ["relative"],
            "tokenTypes": ["label"], "tokenModifiers": ["declaration", "definition"],
        }
    init = request("initialize", {"processId": os.getpid(), "rootUri": root.as_uri(),
                                  "workspaceFolders": [{"uri": root.as_uri(), "name": "project"}],
                                  "initializationOptions": {"indexDir": str(state / "index")},
                                  "capabilities": caps})
    if init.get("serverInfo", {}).get("version") != "ILS-" + RELEASE:
        raise RuntimeError(f"Expected reference version {RELEASE}, received {init.get('serverInfo')}")
    record = {"release": "kotlin-lsp/v" + RELEASE, "registrations": registrations,
              "clientCapabilities": caps, "initialize": init}
    send(process, method="initialized", params={})
    file = (root / "library" if project in GRADLE_PROJECTS else root) / "src/main/kotlin/Example.kt"
    doc = {"uri": file.as_uri()}
    send(process, method="textDocument/didOpen", params={"textDocument": {
        **doc, "languageId": "kotlin", "version": 1, "text": file.read_text()}})
    name = {"position": {"line": 7, "character": 18}}
    for method, extra in [
        ("documentSymbol", {}), ("foldingRange", {}), ("definition", name), ("hover", name),
        ("references", {**name, "context": {"includeDeclaration": True}}),
        ("rename", {**name, "newName": "resultOf"}),
        ("signatureHelp", {"position": {"line": 7, "character": 25}}),
        ("diagnostic", {}), ("completion", {"position": {"line": 7, "character": 19}}),
    ]:
        method = "textDocument/" + method
        record[method] = request(method, {"textDocument": doc, **extra})
    if project:
        consumer = root / "src/main/kotlin/Consumer.kt"
        consumer_doc = {"uri": consumer.as_uri()}
        send(process, method="textDocument/didOpen", params={"textDocument": {
            **consumer_doc, "languageId": "kotlin", "version": 1, "text": consumer.read_text()}})
        at_use = {"textDocument": consumer_doc, "position": {"line": 2, "character": 25}}
        deadline = time.monotonic() + 150
        while True:
            definition = request("textDocument/definition", at_use)
            if isinstance(definition, list) and any(item.get("uri", item.get("targetUri")) == file.as_uri() for item in definition):
                break
            if time.monotonic() > deadline:
                raise RuntimeError(f"{project} import never resolved the cross-file call to Example.kt")
            time.sleep(0.5)
        # Successful navigation to a different file is the import readiness check.
        declaration = {"textDocument": doc, "position": {"line": 3, "character": 5}}
        record = {"release": record["release"], "definition": definition,
                  "hover": request("textDocument/hover", at_use),
                  "references": request("textDocument/references", {
                      **declaration, "context": {"includeDeclaration": True}}),
                  "rename": request("textDocument/rename", {**declaration, "newName": "resultOf"})}
    if project == "editing":
        legend = init["capabilities"]["semanticTokensProvider"]["legend"]
        cases = editing.record_cases(request, notify, buffer, FIXTURES / "editing", legend)
        record = {"release": record["release"], **cases}
    if project == "navigation":
        cases = editing.record_navigation(request, notify, buffer, FIXTURES / "navigation", settings)
        record = {"release": record["release"], **cases}
    if project == "intentions":
        record = {"release": record["release"], **editing.record_intentions(request, notify, buffer, FIXTURES / "intentions", settings)}
    if project == "moves":
        record = {"release": record["release"], "cases": record_moves(root, request)}
    request("shutdown", None)
    send(process, method="exit", params=None)
    experimental = init["capabilities"]["experimental"]
    for key, name in [("indexDir", "index"), ("logFile", "idea.log")]:
        if key in experimental:
            experimental[key] = "<reference-state>/" + name
    text = json.dumps(record, indent=2).replace(doc["uri"], "file:///workspace/Example.kt")
    if project:
        text = text.replace(consumer.as_uri(), "file:///workspace/Consumer.kt")
    if project == "editing":
        text = text.replace(file.with_name("Scopes.kt").as_uri(), "file:///workspace/Scopes.kt")
    return text + "\n"


def move_cases():
    return json.loads((FIXTURES / "moves/cases.json").read_text())


def record_moves(root, request):
    results = {}
    for name, case in move_cases().items():
        paths = {(root / path).as_uri(): path for path in case["files"]}
        moves = [{"oldUri": (root / old).as_uri(), "newUri": (root / dest).as_uri()}
                 for old, dest in case["moves"].items()]
        edit = request("workspace/willRenameFiles", {"files": moves})
        if not edit or "error" in edit:
            raise RuntimeError(f"{name}: expected a workspace edit, received {edit}")
        changes = edit.get("changes", {}).copy()
        for change in edit.get("documentChanges", []):
            changes.setdefault(change["textDocument"]["uri"], []).extend(change["edits"])
        files = case["files"].copy()
        for uri, edits in changes.items():
            path = paths[uri]
            files[path] = editing.apply_edits(files[path], edits)
        if files == case["files"]:
            raise RuntimeError(f"{name}: the move produced no text changes")
        results[name] = files
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    projects = parser.add_mutually_exclusive_group()
    projects.add_argument("--maven", action="store_const", const="maven", dest="project", help="capture Maven cross-file operations")
    projects.add_argument("--gradle", action="store_const", const="gradle", dest="project", help="capture Gradle cross-module operations")
    parser.add_argument("--output", type=Path, required=True)
    projects.add_argument("--editing", action="store_const", const="editing", dest="project", help="capture editing behavior with a binary Kotlin library")
    projects.add_argument("--navigation", action="store_const", const="navigation", dest="project",
                          help="capture overload navigation and lambda and call-chain hints")
    projects.add_argument("--moves", action="store_const", const="moves", dest="project",
                          help="capture package folder and batch workspace edits")
    projects.add_argument("--intentions", action="store_const", const="intentions", dest="project",
                          help="capture outline signatures, type hint links and intentions")
    parser.add_argument("--kc", type=Path, help="kc executable to compile the editing fixture")
    args = parser.parse_args()
    if args.project == "editing" and not args.kc:
        parser.error("--editing requires --kc")
    with tempfile.TemporaryDirectory() as temp:
        state = Path(temp).resolve()
        root = state / "project"
        if args.project in GRADLE_PROJECTS:
            shutil.copytree(FIXTURES / "gradle", root, ignore=shutil.ignore_patterns("upstream-*.json"))
        source = (root / "library" if args.project in GRADLE_PROJECTS else root) / "src/main/kotlin/Example.kt"
        source.parent.mkdir(parents=True)
        source.write_bytes((FIXTURES / "Example.kt").read_bytes())
        if args.project:
            consumer = root / "src/main/kotlin/Consumer.kt"
            consumer.parent.mkdir(parents=True, exist_ok=True)
            consumer.write_bytes((FIXTURES / "Consumer.kt").read_bytes())
        if args.project == "editing":
            fixture = FIXTURES / "editing"
            subprocess.run([args.kc.resolve(), "-no-jdk", "-d", state / "classes", fixture / "Library.kt"], check=True)
            with zipfile.ZipFile(root / "editing.jar", "w") as jar:
                for file in (state / "classes").rglob("*"):
                    if file.is_file():
                        jar.write(file, file.relative_to(state / "classes"))
            with (root / "build.gradle.kts").open("a") as build:
                build.write('\ndependencies { implementation(files("editing.jar")) }\n')
            (root / "gradle.properties").write_text("")
            source.with_name("Scopes.kt").write_bytes((fixture / "Scopes.kt").read_bytes())
            (root / "src/main/kotlin/Editing.kt").write_text("package sample\n")
        if args.project == "intentions":
            shutil.copy(FIXTURES / "intentions/Library.kt", source.parent)
            java = root / "library/src/main/java/library"
            java.mkdir(parents=True)
            shutil.copy(FIXTURES / "intentions/Printer.java", java)
            (root / "gradle.properties").write_text("")
            (root / "src/main/kotlin/Editing.kt").write_text("package sample\n")
        if args.project == "navigation":
            fixture = FIXTURES / "navigation"
            for name in ("Overloads.kt", "IntExtensions.kt"):
                shutil.copy(fixture / name, source.parent)
            java_dir = root / "library/src/main/java/nav"
            java_dir.mkdir(parents=True)
            shutil.copy(fixture / "Parser.java", java_dir)
            (root / "gradle.properties").write_text("")
            (root / "src/main/kotlin/Editing.kt").write_text("package sample\n")
        if args.project == "moves":
            for case in move_cases().values():
                for path, text in case["files"].items():
                    file = root / path
                    file.parent.mkdir(parents=True, exist_ok=True)
                    file.write_text(text)
        if args.project == "maven":
            (root / "pom.xml").write_bytes((FIXTURES / "maven/pom.xml").read_bytes())
        with subprocess.Popen([args.binary, "--stdio", "--system-path=" + str(state / "state")],
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE) as process:
            messages = Queue()
            reader = Thread(target=read_messages, args=(process.stdout, messages), daemon=True)
            reader.start()
            try:
                result = capture(process, messages, root, state, args.project)
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
                reader.join()
        args.output.write_text(result)


if __name__ == "__main__":
    main()
