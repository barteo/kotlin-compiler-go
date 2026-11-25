"""Record editor operations from the pinned JetBrains Kotlin language server."""
import argparse
import json
import os
import shutil
from pathlib import Path
import subprocess
import tempfile
import time
from queue import Queue
from threading import Thread

FIXTURES = Path(__file__).resolve().parents[2] / "internal/lsp/testdata"
RELEASE = "271.2085.0"


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
                    result = [None] * len(msg["params"]["items"])
                if msg["method"] == "client/registerCapability":
                    registrations.extend(msg["params"]["registrations"])
                send(process, id=msg["id"], result=result)

    caps = {"workspace": {"configuration": True, "workspaceFolders": True},
            "textDocument": {"documentSymbol": {"hierarchicalDocumentSymbolSupport": True},
                             "foldingRange": {}, "hover": {"contentFormat": ["markdown"]}}}
    init = request("initialize", {"processId": os.getpid(), "rootUri": root.as_uri(),
                                  "workspaceFolders": [{"uri": root.as_uri(), "name": "project"}],
                                  "initializationOptions": {"indexDir": str(state / "index")},
                                  "capabilities": caps})
    if init.get("serverInfo", {}).get("version") != "ILS-" + RELEASE:
        raise RuntimeError(f"Expected reference version {RELEASE}, received {init.get('serverInfo')}")
    record = {"release": "kotlin-lsp/v" + RELEASE, "registrations": registrations,
              "clientCapabilities": caps, "initialize": init}
    send(process, method="initialized", params={})
    file = (root / "library" if project == "gradle" else root) / "src/main/kotlin/Example.kt"
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
            if isinstance(definition, list) and any(item.get("uri") == file.as_uri() for item in definition):
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
    request("shutdown", None)
    send(process, method="exit", params=None)
    experimental = init["capabilities"]["experimental"]
    for key, name in [("indexDir", "index"), ("logFile", "idea.log")]:
        if key in experimental:
            experimental[key] = "<reference-state>/" + name
    text = json.dumps(record, indent=2).replace(doc["uri"], "file:///workspace/Example.kt")
    if project:
        text = text.replace(consumer.as_uri(), "file:///workspace/Consumer.kt")
    return text + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    projects = parser.add_mutually_exclusive_group()
    projects.add_argument("--maven", action="store_const", const="maven", dest="project", help="capture Maven cross-file operations")
    projects.add_argument("--gradle", action="store_const", const="gradle", dest="project", help="capture Gradle cross-module operations")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    with tempfile.TemporaryDirectory() as temp:
        state = Path(temp).resolve()
        root = state / "project"
        if args.project == "gradle":
            shutil.copytree(FIXTURES / "gradle", root, ignore=shutil.ignore_patterns("upstream-*.json"))
        source = (root / "library" if args.project == "gradle" else root) / "src/main/kotlin/Example.kt"
        source.parent.mkdir(parents=True)
        source.write_bytes((FIXTURES / "Example.kt").read_bytes())
        if args.project:
            consumer = root / "src/main/kotlin/Consumer.kt"
            consumer.parent.mkdir(parents=True, exist_ok=True)
            consumer.write_bytes((FIXTURES / "Consumer.kt").read_bytes())
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
