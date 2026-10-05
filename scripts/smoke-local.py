"""Small HTTP smoke check against an already-running disposable local stack."""
import re
import sys
import time
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

    target = "https://example.com/local-smoke-" + uuid.uuid4().hex + "?check=1"
    request = Request(base + "/shorten", data=urlencode({"url": target}).encode())
    with client.open(request, timeout=10) as response:
        body = response.read().decode()
        match = re.search(r'href="(/r/[A-Za-z0-9]+)"', body)
        if response.status != 200 or not match:
            raise RuntimeError("Create did not return a short URL")

    for state in ("cold", "warm"):
        # Keep this setup smoke check below the current shared limiter's rate.
        time.sleep(0.6)
        try:
            client.open(base + match.group(1), timeout=10)
        except HTTPError as response:
            if response.code != 308 or response.headers.get("Location") != target:
                raise RuntimeError(f"Unexpected {state} redirect: {response.code}")
            response.close()
        else:
            raise RuntimeError(f"Expected a 308 {state} redirect")

    print(f"PASS: form, create, cold/warm 308 redirects through Caddy ({match.group(1)})")
    print("Smoke check only; this does not establish browser JS, cache use, or failure recovery.")


if __name__ == "__main__":
    main()
