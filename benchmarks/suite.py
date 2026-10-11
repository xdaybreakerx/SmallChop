#!/usr/bin/env python3
"""Build, seed, measure and clean up an isolated Mongo/cache benchmark stack."""

import argparse
import fcntl
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import socket
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request

from report import write_report, qualify
from run import positive

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
ALPHABET = "bcdfghjklmnpqrstvwxyzBCDFGHJKLMNPQRSTVWXYZ0123456789"


class SuiteLock:
    def __init__(self, project):
        self.path = Path(tempfile.gettempdir()) / (project + ".lock")

    def __enter__(self):
        self.file = self.path.open("a")
        try:
            fcntl.flock(self.file, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            self.file.close()
            raise RuntimeError("Another local suite is using this project") from None
        return self

    def __exit__(self, *args):
        self.file.close()


def source_digest():
    paths = [ROOT / name for name in ("go.mod", "go.sum", "Dockerfile")]
    paths += list((ROOT / "cmd").rglob("*.go")) + list((ROOT / "internal").rglob("*.go"))
    paths += [p for p in (ROOT / "internal/templates").rglob("*") if p.is_file()]
    digest = hashlib.sha256()
    for path in sorted(paths):
        digest.update(str(path.relative_to(ROOT)).encode() + b"\0" + path.read_bytes() + b"\0")
    return digest.hexdigest()


def fixtures(count):
    def encode(number):
        code = ""
        while number:
            number, digit = divmod(number, len(ALPHABET))
            code = ALPHABET[digit] + code
        return code
    return [dict(id=n, code=encode(n), destination=f"https://example.com/benchmark/{n}/a%2Fb?x=1&x=2#section")
            for n in range(1, count + 1)]


def plan(rates, repetitions):
    result = []
    for repetition in range(1, repetitions + 1):
        for index, rate in enumerate(rates):
            modes = ["mongo-only", "mongo-redis"]
            if (repetition + index) % 2 == 0:
                modes.reverse()
            result.extend(dict(rate=rate, repetition=repetition, mode=mode) for mode in modes)
    return result


def parse_info(raw):
    return dict(line.split(":", 1) for line in raw.splitlines() if line and not line.startswith("#") and ":" in line)


class Stack:
    def __init__(self, args, work, output):
        self.args, self.work, self.output = args, work, output
        self.env_file = work / "benchmark.env"
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            self.port = sock.getsockname()[1]
        self.target = f"http://127.0.0.1:{self.port}"
        self.prefix = ["docker", "compose", "--project-directory", str(ROOT),
                       "--env-file", str(self.env_file), "--project-name", args.project,
                       "-f", "deploy/compose.local.yml", "-f", "benchmarks/compose.yml"]
        self.log = (output / "lifecycle.log").open("w")
        self.started = False
        # Never inherit Compose selectors or interpolation from the user's shell.
        self.process_env = {k: v for k, v in os.environ.items()
                            if not k.startswith(("COMPOSE_", "BENCH_", "LOCAL_", "MONGO_", "REDIS_", "CREATE_", "REDIRECT_"))}

    def run(self, command, timeout=180, record=True):
        result = subprocess.run(command, cwd=ROOT, env=self.process_env,
                                capture_output=True, text=True, timeout=timeout)
        if record:
            self.log.write(result.stdout + result.stderr)
        self.log.flush()
        if result.returncode:
            raise RuntimeError(f"Command failed ({result.returncode}): {' '.join(command[:3])}; see lifecycle.log")
        return result.stdout.strip()

    def compose(self, *args, timeout=180):
        return self.run(self.prefix + list(args), timeout)

    def configure(self, mode):
        settings = (ROOT / "deploy/env/local.env.example").read_text()
        values = {"LOCAL_HTTP_PORT": str(self.port), "LOCAL_PROXY_SUBNET": "172.30.83.0/29",
                  "LOCAL_PROXY_IP": "172.30.83.2", "LOCAL_APP_PROXY_IP": "172.30.83.3",
                  "REDIRECT_RATE_PER_SECOND": str(max(max(self.args.rates) * 10, self.args.dataset * 2)),
                  "REDIRECT_RATE_BURST": str(max(max(self.args.rates) * 10, self.args.dataset * 2)),
                  "BENCH_CACHE_ENABLED": "true" if mode == "mongo-redis" else "false",
                  "BENCH_ENV_FILE": str(self.env_file)}
        lines = [line for line in settings.splitlines() if line.split("=", 1)[0] not in values]
        self.env_file.write_text("\n".join(lines + [f"{key}={value}" for key, value in values.items()]) + "\n")
        self.settings = values

    def start(self):
        version = self.run(["docker", "compose", "version", "--short"])
        numbers = tuple(int(n) for n in re.findall(r"\d+", version)[:3])
        if numbers < (2, 24, 4):
            raise RuntimeError("Compose 2.24.4+ is required")
        for kind, command in [("containers", ["ps", "-aq"]), ("volumes", ["volume", "ls", "-q"]),
                              ("networks", ["network", "ls", "-q"])]:
            if self.run(["docker"] + command + ["--filter", f"label=com.docker.compose.project={self.args.project}"]):
                raise RuntimeError(f"Existing benchmark {kind}: refusing reuse/deletion")
        self.configure("mongo-only")
        self.compose("config", "--quiet")
        self.started = True  # Clean up partial startup too.
        self.compose("up", "--build", "-d", "--wait", "--wait-timeout", "120", timeout=600)

    def redis(self, *args):
        return self.compose("exec", "-T", "redis", "sh", "-c",
                            'REDISCLI_AUTH="$REDIS_PASSWORD" exec redis-cli --raw "$@"', "sh", *args)

    def mongo(self, js):
        return self.compose("exec", "-T", "mongo", "mongosh", "--quiet", "--eval",
                            "const admin = db.getSiblingDB('admin'); "
                            "if (!admin.auth(process.env.MONGO_INITDB_ROOT_USERNAME, process.env.MONGO_INITDB_ROOT_PASSWORD)) throw new Error('auth'); " + js)

    def seed(self, rows):
        seed = self.work / "fixtures.json"
        seed.write_text(json.dumps(rows))
        self.compose("cp", str(seed), "mongo:/tmp/benchmark-fixtures.json")
        self.compose("cp", str(HERE / "seed.js"), "mongo:/tmp/benchmark-seed.js")
        result = json.loads(self.compose("exec", "-T", "mongo", "mongosh", "--quiet", "/tmp/benchmark-seed.js"))
        if result["count"] != len(rows):
            raise RuntimeError("Seed count mismatch")
        (self.output / "seed.json").write_text(json.dumps(result, indent=2) + "\n")

    def counters(self):
        mongo = json.loads(self.mongo("const s = admin.runCommand({serverStatus: 1}); "
                                     "if (!s.ok) throw new Error('serverStatus'); "
                                     "print(JSON.stringify({mongo_find: Number(s.metrics.commands.find?.total || 0), mongo_uptime: s.uptime}));"))
        info = parse_info(self.redis("INFO", "stats", "commandstats", "server"))
        def calls(command):
            values = dict(item.split("=", 1) for item in info.get("cmdstat_" + command, "calls=0").split(","))
            return int(values["calls"])
        return dict(**mongo, redis_get=calls("get"), redis_set=calls("set"),
                    redis_hits=int(info["keyspace_hits"]), redis_misses=int(info["keyspace_misses"]),
                    redis_run_id=info["run_id"])

    def warm_all(self, rows):
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, *args):
                return None
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        for row in rows:
            try:
                response = opener.open(self.target + "/r/" + row["code"], timeout=5)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                if response.code != 308 or response.headers.get("Location") != row["destination"]:
                    raise RuntimeError("Warm-up redirect failed exact status/Location validation")

    def identities(self):
        result = {}
        for service in ("app", "mongo", "redis", "caddy"):
            container = self.compose("ps", "-q", service)
            # Inspect only selected non-sensitive fields, never archive environment credentials.
            image = self.run(["docker", "inspect", "--format", '{{.Image}}', container])
            limits = json.loads(self.run(["docker", "inspect", "--format",
                                         '{"NanoCpus":{{.HostConfig.NanoCpus}},"Memory":{{.HostConfig.Memory}}}', container]))
            result[service] = dict(container_id=container, image_id=image,
                                   nano_cpus=limits["NanoCpus"], memory_bytes=limits["Memory"])
        return result

    def app_settings(self):
        container = self.compose("ps", "-q", "app")
        env = json.loads(self.run(["docker", "inspect", "--format", "{{json .Config.Env}}", container], record=False))
        wanted = {"CACHE_ENABLED", "PUBLIC_BASE_URL", "TRUSTED_PROXY_IPS", "REDIRECT_RATE_PER_SECOND", "REDIRECT_RATE_BURST",
                  "REQUEST_TIMEOUT", "MONGO_TIMEOUT", "CACHE_TIMEOUT", "STARTUP_TIMEOUT"}
        return dict(line.split("=", 1) for line in env if line.split("=", 1)[0] in wanted)

    def close(self):
        try:
            if self.started:
                self.compose("down", "--volumes", "--remove-orphans")
        finally:
            self.log.close()


def workload(args, stack, fixture_file, directory, label, duration, containers=None):
    command = [sys.executable, str(HERE / "run.py"), "--k6", args.k6,
                             "--target", stack.target, "--fixtures", str(fixture_file),
                             "--label", label, "--rate", str(args.current_rate), "--duration", str(duration),
                             "--vus", str(args.vus), "--timeout-ms", str(args.timeout_ms),
                             "--output", str(directory)]
    if containers:
        command += ["--containers", *containers]
    result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True,
                            timeout=duration + args.timeout_ms / 1000 + 40)
    if not (directory / "summary.json").exists():
        raise RuntimeError("k6 failed before producing results: " + result.stderr[-1000:])
    return result.returncode


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--k6", default="k6")
    parser.add_argument("--check", action="store_true", help="short functional integration; not performance evidence")
    parser.add_argument("--rates", nargs="+", type=positive)
    parser.add_argument("--duration", type=positive)
    parser.add_argument("--warmup", type=positive)
    parser.add_argument("--repetitions", type=positive)
    parser.add_argument("--dataset", type=positive)
    parser.add_argument("--vus", type=positive, default=100)
    parser.add_argument("--timeout-ms", type=positive, default=3000)
    parser.add_argument("--project", default="smallchop-local-benchmark")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r"smallchop-local-benchmark(?:-[a-z0-9-]+)?", args.project):
        parser.error("project must be smallchop-local-benchmark or use its hyphenated suffix")
    args.rates = args.rates or ([20, 40] if args.check else [100, 500, 1000])
    if len(set(args.rates)) != len(args.rates):
        parser.error("rates must be distinct")
    args.duration = args.duration or (2 if args.check else 30)
    args.warmup = args.warmup or (1 if args.check else 5)
    args.repetitions = args.repetitions or (2 if args.check else 3)
    args.dataset = args.dataset or (64 if args.check else 1000)
    pinned = (HERE / ".k6-version").read_text().strip()
    version = subprocess.check_output([args.k6, "version"], text=True).strip()
    if not version.startswith(f"k6 v{pinned} "):
        raise ValueError(f"k6 {pinned} required")
    output = (args.output or HERE / "results" / datetime.now(timezone.utc).strftime("suite-%Y%m%dT%H%M%S.%fZ")).resolve()
    output.mkdir(parents=True, exist_ok=False)
    rows = fixtures(args.dataset)
    fixture_file = output / "fixtures.json"
    fixture_file.write_text(json.dumps(rows, indent=2) + "\n")
    config = {key: getattr(args, key) for key in ("rates", "duration", "warmup", "repetitions", "dataset", "vus", "timeout_ms", "check", "project")}
    suite = dict(schema_version=1, config=config, plan=plan(args.rates, args.repetitions), measurements=[],
                 source_revision=subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
                 source_dirty=bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True)),
                 application_source_sha256=source_digest(),
                 fixture_sha256=hashlib.sha256(fixture_file.read_bytes()).hexdigest(), k6_version=version,
                 generator=dict(system=platform.system(), architecture=platform.machine(), kernel=platform.release(), logical_cpus=os.cpu_count()),
                 started_at_utc=datetime.now(timezone.utc).isoformat())
    def save():
        (output / "suite.json").write_text(json.dumps(suite, indent=2) + "\n")
    save()
    with SuiteLock(args.project), tempfile.TemporaryDirectory(prefix="smallchop-benchmark-") as temporary:
        stack = Stack(args, Path(temporary), output)
        try:
            print(f"Starting fresh {args.project}; evidence: {output}", flush=True)
            stack.start()
            if source_digest() != suite["application_source_sha256"]:
                raise RuntimeError("Application sources changed during the build; rerun on a stable checkout")
            suite["docker"] = dict(version=stack.run(["docker", "version", "--format", "{{.Server.Version}}"]),
                                   cpus=int(stack.run(["docker", "info", "--format", "{{.NCPU}}"])),
                                   memory_bytes=int(stack.run(["docker", "info", "--format", "{{.MemTotal}}"])))
            suite["images_and_limits"] = stack.identities()
            stack.seed(rows)
            for index, point in enumerate(suite["plan"], 1):
                mode, args.current_rate = point["mode"], point["rate"]
                label = f"{index:02d}-{mode}-r{point['rate']}-rep{point['repetition']}"
                print(f"[{index}/{len(suite['plan'])}] {label}", flush=True)
                stack.configure(mode)
                stack.compose("up", "--no-deps", "--force-recreate", "-d", "--wait", "--wait-timeout", "60", "app")
                stack.redis("FLUSHDB")
                stack.warm_all(rows)
                key_count = int(stack.redis("DBSIZE"))
                if key_count != (len(rows) if mode == "mongo-redis" else 0):
                    raise RuntimeError("Warm-up cache population contradicts configured mode")
                # Retain failing high-load warm-ups too; the measured point will remain unqualified.
                warmup_exit = workload(args, stack, fixture_file, output / (label + "-warmup"), label + "-warmup", args.warmup)
                identities = stack.identities()
                if identities["app"]["image_id"] != suite["images_and_limits"]["app"]["image_id"]:
                    raise RuntimeError("Application image changed during the experiment")
                app_settings = stack.app_settings()
                if app_settings["CACHE_ENABLED"] != ("true" if mode == "mongo-redis" else "false"):
                    raise RuntimeError("Application cache configuration does not match the planned mode")
                before = stack.counters()
                directory = output / label
                workload(args, stack, fixture_file, directory, label, args.duration,
                         [info["container_id"] for info in identities.values()])
                after = stack.counters()
                (directory / "backend.json").write_text(json.dumps(dict(before=before, after=after), indent=2) + "\n")
                observation = qualify(directory, mode, before, after)
                if warmup_exit:
                    observation["qualified"] = False
                    observation["reasons"].append("offered-rate warm-up thresholds failed")
                (directory / "application.json").write_text(json.dumps(dict(config=app_settings, identities=identities), indent=2) + "\n")
                observation.update(rate=point["rate"], repetition=point["repetition"], directory=label)
                suite["measurements"].append(observation)
                save()
                print(f"  {'qualified' if observation['qualified'] else 'UNQUALIFIED'}; Mongo finds={observation['backend_delta']['mongo_find']}, Redis hits={observation['backend_delta']['redis_hits']}", flush=True)
        except (Exception, KeyboardInterrupt) as error:
            suite["error"] = f"{type(error).__name__}: {error}"
            raise
        finally:
            try:
                stack.close()
                suite["cleanup"] = "completed" if stack.started else "no resources created"
            except Exception as error:
                suite["cleanup"] = f"FAILED: {error}"
                raise
            finally:
                suite["finished_at_utc"] = datetime.now(timezone.utc).isoformat()
                save()
                write_report(output)
    result = write_report(output)
    print(f"Report: {output / 'comparison.md'}", flush=True)
    return 0 if result["qualified"] else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (Exception, KeyboardInterrupt) as error:
        print(f"benchmark suite: {error}", file=sys.stderr)
        sys.exit(1)
