#!/usr/bin/env python3
"""Exercise reliability and observability in a fresh, self-cleaning Compose project (Compose 2.24.4+)."""
import fcntl
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


def check_dependencies():
    observations = []
    metric_checks = []
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
            req.add_header("X-Request-ID", "caller-private-id")
            start = time.monotonic()
            try:
                response = opener.open(req, timeout=5)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                body = response.read().decode()
                actual, location = response.code, response.headers.get("Location")
                request_id = response.headers.get("X-Request-ID")
            elapsed = time.monotonic() - start
            assert re.fullmatch(r"[0-9a-f]{32}", request_id or ""), f"{path}: missing server request ID"
            assert actual == status, f"{path}: status {actual}, expected {status}"
            if destination is not None:
                assert location == destination, f"{path}: changed destination {location!r}"
            assert elapsed < 3.5, f"{path}: exceeded dependency budget ({elapsed:.3f}s)"
            observations.append({"path": path, "status": actual, "seconds": round(elapsed, 3), "request_id": request_id})
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

        def metrics():
            text = compose("exec", "-T", "app", "wget", "-q", "-O", "-",
                           "http://127.0.0.1:9090/metrics")
            samples = {}
            for line in text.splitlines():
                match = re.fullmatch(r'([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*?)\})? ([^ ]+)(?: .*)?', line)
                if not match:
                    continue
                labels = tuple(sorted(re.findall(r'([a-z_]+)="([^"]*)"', match[2] or "")))
                samples[(match[1], labels)] = float(match[3])
            return samples

        def value(samples, name, **labels):
            return samples.get(("smallchop_" + name, tuple(sorted(labels.items()))), 0)

        def check_delta(before, after, name, count, **labels):
            delta = value(after, name, **labels) - value(before, name, **labels)
            assert delta == count, f"{name} {labels}: delta {delta}, expected {count}"
            metric_checks.append({"metric": name, "labels": labels, "delta": delta})

        def check_safe_logs():
            output = compose("logs", "--no-log-prefix", "app")
            logs = []
            for line in output.splitlines():
                if line.startswith("{"):
                    logs.append(json.loads(line))
            completed = {row["request_id"]: row for row in logs if row.get("msg") == "http request completed"}
            for observation in observations:
                row = completed.get(observation["request_id"])
                assert row and row["status"] == observation["status"], "missing/mismatched correlated request log"
                assert row["duration_seconds"] >= 0
                assert row["route"] in {"/", "/shorten", "/r/{code}", "/livez", "/readyz", "unmatched"}
            assert not any(secret in output for secret in ["log-privacy-secret", "cache-offline#fragment", "a%2Fb", "caller-private-id"])
            return len(completed)

        result = {"result": "INCOMPLETE", "project": PROJECT}
        try:
            compose("up", "--build", "-d", "--wait", "--wait-timeout", "120")
            request("/livez", 200)
            request("/readyz", 200)
            request("/metrics", 404)  # Metrics are not routed through Caddy.
            first = "https://example.com/a%2Fb?q=a%2Bb&x=2&x=1&token=log-privacy-secret#installation"
            code = create(first)
            mongo('appdb.urls.updateOne({_id:1}, {$set:{accessCount:7}})')
            before = metrics()
            request("/r/" + code, 308, first)  # Cold read and fill.
            cold = metrics()
            check_delta(before, cold, "cache_reads_total", 1, outcome="miss")
            check_delta(before, cold, "cache_fills_total", 1, outcome="success")
            check_delta(before, cold, "dependency_failures_total", 0, dependency="redis", operation="get")
            request("/r/" + code, 308, first)  # Warm read.
            warm = metrics()
            check_delta(cold, warm, "cache_reads_total", 1, outcome="hit")
            check_delta(before, warm, "http_requests_total", 2, route="/r/{code}", method="GET", status_class="3xx")
            check_delta(before, warm, "http_request_duration_seconds_count", 2, route="/r/{code}", method="GET", status_class="3xx")
            assert value(warm, "http_request_duration_seconds_sum", route="/r/{code}", method="GET", status_class="3xx") >= 0
            assert redis("GET", code) == first
            assert mongo('print(appdb.urls.findOne({_id:2}))') == "null"
            assert mongo('print(appdb.urls.findOne({_id:1}).accessCount)') == "7"
            request("/r/999999", 404)

            before_absent = metrics()
            request("/r/999999", 404)
            after_absent = metrics()
            check_delta(before_absent, after_absent, "dependency_failures_total", 0, dependency="mongo", operation="find")
            check_delta(before_absent, after_absent, "cache_reads_total", 1, outcome="miss")

            compose("stop", "redis")
            request("/livez", 200)
            request("/readyz", 200)  # Redis is optional.
            second = "https://example.org/cache-offline#fragment"
            second_code = create(second)
            assert mongo('print("accessCount" in appdb.urls.findOne({_id:2}))') == "false"
            offline = metrics()
            request("/r/" + second_code, 308, second)
            fallback = metrics()
            check_delta(offline, fallback, "cache_reads_total", 1, outcome="error")
            check_delta(offline, fallback, "cache_fills_total", 1, outcome="error")
            check_delta(offline, fallback, "dependency_failures_total", 1, dependency="redis", operation="get")
            check_delta(offline, fallback, "dependency_failures_total", 1, dependency="redis", operation="set")
            log_count = check_safe_logs()
            initial_observations = list(observations)
            compose("up", "--no-deps", "--force-recreate", "-d", "app")
            wait_http()  # The app starts while Redis remains stopped.
            observations.clear()  # The recreated process has a new log/counter lifetime.
            app_before = compose("ps", "-q", "app")
            process_before = json.loads(run(["docker", "inspect", "--format", "{{json .State}}", app_before]))
            request("/readyz", 200)
            request("/r/" + second_code, 308, second)

            compose("up", "--no-deps", "-d", "--wait", "redis")
            recovery_before = metrics()
            request("/r/" + second_code, 308, second)
            recovered = metrics()
            check_delta(recovery_before, recovered, "cache_reads_total", 1, outcome="miss")
            check_delta(recovery_before, recovered, "cache_fills_total", 1, outcome="success")
            request("/r/" + second_code, 308, second)
            check_delta(recovered, metrics(), "cache_reads_total", 1, outcome="hit")
            assert redis("GET", second_code) == second
            app_after = compose("ps", "-q", "app")
            process_after = json.loads(run(["docker", "inspect", "--format", "{{json .State}}", app_after]))
            assert app_before == app_after and process_before["StartedAt"] == process_after["StartedAt"] and process_before["Pid"] == process_after["Pid"], "app restarted during cache recovery"
            redis("DEL", code)
            compose("stop", "mongo")
            before_mongo = metrics()
            request("/livez", 200)
            request("/readyz", 503)
            check_delta(before_mongo, metrics(), "dependency_failures_total", 1, dependency="mongo", operation="ping")
            request("/r/" + second_code, 308, second)  # Cached path needs no MongoDB.
            request("/r/" + code, 503)  # Required cold read is unavailable, not absent.
            request("/shorten", 503, form="https://example.net/mongo-offline")
            mongo_failed = metrics()
            check_delta(before_mongo, mongo_failed, "dependency_failures_total", 1, dependency="mongo", operation="find")
            check_delta(before_mongo, mongo_failed, "dependency_failures_total", 1, dependency="mongo", operation="save")
            check_delta(before_mongo, mongo_failed, "http_requests_total", 1, route="/readyz", method="GET", status_class="5xx")
            log_count += check_safe_logs()
            # Keep both process lifetimes' observations, not just the final restart.
            recovery_observations = list(observations)

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
            observations.clear()
            request("/livez", 200)
            request("/readyz", 200)
            request("/r/" + code, 308, first)
            assert mongo('print(appdb.urls.findOne({_id:1}).accessCount)') == "7"
            log_count += check_safe_logs()
            result = {"result": "PASS", "project": PROJECT, "requests": initial_observations + recovery_observations + observations,
                              "metric_checks": metric_checks, "correlated_logs": log_count,
                              "health_dependency_contract": True, "private_metrics_listener": True,
                              "optional_redis_startup": True, "required_mongo_startup_bounded": True,
                              "cache_recovery_without_app_restart": True, "legacy_count_unchanged": True}
        except Exception as error:
            result.update(result="FAIL", error=str(error), requests=observations, metric_checks=metric_checks)
            raise
        finally:
            try:
                compose("down", "--volumes")  # Only this newly created disposable project.
                result["cleanup"] = "completed"
            except Exception:
                result.update(result="FAIL", cleanup="failed")
                raise
            finally:
                # Preserve partial CI evidence on assertion errors; PASS requires cleanup too.
                print(json.dumps(result, indent=2))


def main():
    with (Path(tempfile.gettempdir()) / f"{PROJECT}.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise RuntimeError(f"Another local {PROJECT} check is running") from error
        check_dependencies()


if __name__ == "__main__":
    main()
