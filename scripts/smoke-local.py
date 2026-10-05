"""Small HTTP smoke check against an already-running disposable local stack."""
import re
import sys
import uuid
from urllib.error import HTTPError
from urllib.parse import urlencode, urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    base = sys.argv[1].rstrip("/")
    parsed = urlparse(base)
    if parsed.scheme != "http" or parsed.hostname != "127.0.0.1":
        raise ValueError("Smoke checks only target the loopback local stack")
    client = build_opener(NoRedirect())
    with client.open(base + "/", timeout=10) as response:
        if response.status != 200 or b"<form" not in response.read():
            raise RuntimeError("Root page did not serve the form")

    target = "https://example.com/a%2Fb/local-smoke-" + uuid.uuid4().hex + "?q=a%2Bb&x=2&x=1#installation"
    request = Request(base + "/shorten", data=urlencode({"url": target}).encode())
    request.add_header("Host", "caller-controlled.example")
    request.add_header("X-Forwarded-Host", "caller-controlled.example")
    with client.open(request, timeout=10) as response:
        body = response.read().decode()
        match = re.search(r'href="(/r/[A-Za-z0-9]+)"', body)
        if response.status != 200 or not match or base + match.group(1) not in body:
            raise RuntimeError("Create did not return a short URL")
        if "caller-controlled.example" in body:
            raise RuntimeError("Caller headers changed the public origin")

    for state in ("cold", "warm"):
        try:
            client.open(base + match.group(1), timeout=10)
        except HTTPError as response:
            if response.code != 308 or response.headers.get("Location") != target:
                raise RuntimeError(f"Unexpected {state} redirect: {response.code}")
            response.close()
        else:
            raise RuntimeError(f"Expected a 308 {state} redirect")

    print(f"PASS: form, configured origin, exact query/fragment and cold/warm 308 redirects through Caddy ({match.group(1)})")
    print("Smoke check only; this does not establish browser JS, cache use, or failure recovery.")


if __name__ == "__main__":
    main()
