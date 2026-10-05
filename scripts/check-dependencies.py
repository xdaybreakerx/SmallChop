#!/usr/bin/env python3
"""Exercise P2 in a fresh, self-cleaning Compose project (Compose 2.24.4+)."""
import json
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
PROJECT = "smallchop-local-dependency-check"


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def run(args, **kwargs):
    return subprocess.run(args, cwd=ROOT, check=True, text=True, capture_output=True, **kwargs).stdout.strip()


def main():
    observations = []
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    base = f"http://127.0.0.1:{port}"

    with tempfile.TemporaryDirectory(prefix="smallchop-p2-") as temporary:
        work = Path(temporary)
        env = work / "test.env"
        settings = (ROOT / "deploy/env/local.env.example").read_text()
        settings = settings.replace("LOCAL_HTTP_PORT=8080", f"LOCAL_HTTP_PORT={port}")
        settings = settings.replace("172.30.80.", "172.30.82.")
        env.write_text(settings.replace("STARTUP_TIMEOUT=10s", "STARTUP_TIMEOUT=2s"))
        override = work / "test.yml"
        override.write_text("services:\n" + "".join(
            f"  {service}:\n    env_file: !override [{json.dumps(str(env))}]\n"
            for service in ["app", "mongo", "redis"]
        ))
        command = ["docker", "compose", "--project-directory", str(ROOT), "--env-file", str(env),
                   "--project-name", PROJECT, "-f", "deploy/compose.local.yml", "-f", str(override)]

        def compose(*args):
            return run(command + list(args), timeout=180)

        # Never attach to or delete an existing experiment's containers/storage.
        if run(["docker", "ps", "-aq", "--filter", f"label=com.docker.compose.project={PROJECT}"]):
            raise RuntimeError(f"Existing {PROJECT} containers; choose/clean that experiment explicitly first")
        if run(["docker", "volume", "ls", "-q", "--filter", f"label=com.docker.compose.project={PROJECT}"]):
            raise RuntimeError(f"Existing {PROJECT} storage; refusing to reuse or delete it")

        if run(["docker", "network", "ls", "-q", "--filter", f"label=com.docker.compose.project={PROJECT}"]):
            raise RuntimeError(f"Existing {PROJECT} network; refusing to reuse or delete it")

        def request(path, status, destination=None, form=None):
            data = urllib.parse.urlencode({"url": form}).encode() if form else None
            req = urllib.request.Request(base + path, data=data)
            start = time.monotonic()
            try:
                response = opener.open(req, timeout=5)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                body = response.read().decode()
                actual, location = response.code, response.headers.get("Location")
            elapsed = time.monotonic() - start
            assert actual == status, f"{path}: status {actual}, expected {status}"
            if destination is not None:
                assert location == destination, f"{path}: changed destination {location!r}"
            assert elapsed < 3.5, f"{path}: exceeded dependency budget ({elapsed:.3f}s)"
            observations.append({"path": path, "status": actual, "seconds": round(elapsed, 3)})
            return body

        def wait_http():
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                try:
                    with opener.open(base, timeout=1) as response:
                        if response.code == 200:
                            return
                except (OSError, urllib.error.URLError):
                    pass
                time.sleep(0.2)
            raise RuntimeError("app did not serve HTTP after restart")

        def create(destination):
            body = request("/shorten", 200, form=destination)
            return re.search(r'href="/r/([^"]+)"', body).group(1)

        def redis(*args):
            return compose("exec", "-T", "redis", "sh", "-c",
                           'REDISCLI_AUTH="$REDIS_PASSWORD" exec redis-cli "$@"', "sh", *args)

        def mongo(js):
            return compose("exec", "-T", "mongo", "mongosh", "--quiet", "--eval",
                           'const appdb = db.getSiblingDB(process.env.MONGO_DB_NAME); '
                           'appdb.auth(process.env.MONGO_APP_USERNAME, process.env.MONGO_APP_PASSWORD); ' + js)

        try:
            compose("up", "--build", "-d", "--wait", "--wait-timeout", "120")
            first = "https://example.com/a%2Fb?q=a%2Bb&x=2&x=1#installation"
            code = create(first)
            mongo('appdb.urls.updateOne({_id:1}, {$set:{accessCount:7}})')
            request("/r/" + code, 308, first)  # Cold read and fill.
            request("/r/" + code, 308, first)  # Warm read.
            assert redis("GET", code) == first
            assert mongo('print(appdb.urls.findOne({_id:2}))') == "null"
            assert mongo('print(appdb.urls.findOne({_id:1}).accessCount)') == "7"
            request("/r/999999", 404)

            compose("stop", "redis")
            second = "https://example.org/cache-offline#fragment"
            second_code = create(second)
            assert mongo('print("accessCount" in appdb.urls.findOne({_id:2}))') == "false"
            request("/r/" + second_code, 308, second)
            compose("up", "--no-deps", "--force-recreate", "-d", "app")
            wait_http()  # The app starts while Redis remains stopped.
            request("/r/" + second_code, 308, second)

            compose("up", "--no-deps", "-d", "--wait", "redis")
            request("/r/" + second_code, 308, second)
            assert redis("GET", second_code) == second  # Caching resumes on the same app process.
            redis("DEL", code)
            compose("stop", "mongo")
            request("/r/" + second_code, 308, second)  # Cached path needs no MongoDB.
            request("/r/" + code, 503)  # Required cold read is unavailable, not absent.
            request("/shorten", 503, form="https://example.net/mongo-offline")

            compose("up", "--no-deps", "--force-recreate", "-d", "app")
            app = compose("ps", "-aq", "app")
            deadline = time.monotonic() + 8
            while time.monotonic() < deadline:
                state = json.loads(run(["docker", "inspect", "--format", "{{json .State}}", app]))
                if not state["Running"]:
                    assert state["ExitCode"] != 0
                    break
                time.sleep(0.2)
            else:
                raise RuntimeError("app did not exit within bounded required-Mongo startup")
            compose("up", "--no-deps", "-d", "--wait", "mongo")
            compose("up", "--no-deps", "--force-recreate", "-d", "app")
            wait_http()
            request("/r/" + code, 308, first)
            assert mongo('print(appdb.urls.findOne({_id:1}).accessCount)') == "7"
            print(json.dumps({"result": "PASS", "project": PROJECT, "requests": observations,
                              "optional_redis_startup": True, "required_mongo_startup_bounded": True,
                              "cache_recovery_without_app_restart": True, "legacy_count_unchanged": True}, indent=2))
        finally:
            compose("down", "--volumes")  # Only this newly created disposable project.


if __name__ == "__main__":
    main()
