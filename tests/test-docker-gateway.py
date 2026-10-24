#!/usr/bin/env python3
"""Check local and tunnel ingress without opening a public tunnel."""
import base64
import datetime
import json
from http.cookies import SimpleCookie
import os
from pathlib import Path
import secrets
import re
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def main():
    # CI shares the project directory with its sibling Docker daemon, not /tmp.
    with tempfile.TemporaryDirectory(prefix=".gateway-test-", dir=ROOT) as directory:
        work = Path(directory)
        (work / "auth").mkdir(mode=0o700)
        (work / "auth" / "credentials").write_text("Username: review\nPassword: review-password\n")
        (work / "auth" / "session-key").write_bytes(secrets.token_bytes(32))
        (work / "app.go").write_text('''package main
import("encoding/json";"net/http")
func main(){http.ListenAndServe(":8080",http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){json.NewEncoder(w).Encode(map[string]string{"path":r.URL.RequestURI(),"host":r.Host,"authorization":r.Header.Get("Authorization"),"proto":r.Header.Get("X-Forwarded-Proto")})}))}
''')
        image = os.environ.get("GATEWAY_AUTH_IMAGE", "golang:1.26-alpine")
        subprocess.run([
            "docker", "run", "--rm", "--network", "none", "-e", "CGO_ENABLED=0", "-e", "GO111MODULE=off",
            "-v", str(ROOT / "mount/dev/compose/traefik/mount/auth/main.go") + ":/src/main.go:ro",
            "-v", str(work) + ":/out", image, "sh", "-ec",
            "go build -o /out/gateway /src/main.go; go build -o /out/app /out/app.go",
        ], check=True)
        (work / "dynamic.yml").write_text('''http:
  routers:
    application:
      rule: PathPrefix(`/`)
      entryPoints: [web]
      service: application
  services:
    application:
      loadBalancer:
        servers:
          - url: http://dashboard:8080
''')
        (work / "compose.yml").write_text('''services:
  traefik:
    image: ${TRAEFIK_IMAGE:-traefik:v3.6.16}
    command: ["--entrypoints.web.address=:8080", "--entrypoints.web.forwardedheaders.insecure=true", "--providers.file.filename=/dynamic.yml"]
    ports: ["${GATEWAY_TEST_BIND_ADDR:-127.0.0.1}::8080"]
    volumes: ["./dynamic.yml:/dynamic.yml:ro"]
    networks: [local, control]
  dashboard:
    image: ${GATEWAY_AUTH_IMAGE:-golang:1.26-alpine}
    command: [/app]
    volumes: ["./app:/app:ro"]
    networks: [local]
  gateway-auth:
    image: ${GATEWAY_AUTH_IMAGE:-golang:1.26-alpine}
    profiles: [tunnel]
    restart: "no"
    command: [/gateway]
    environment:
      GATEWAY_UPSTREAM: http://traefik:8080
    # Only the test publishes ingress; production tunnels reach it internally.
    ports: ["${GATEWAY_TEST_BIND_ADDR:-127.0.0.1}::8080"]
    volumes: ["./gateway:/gateway:ro", "./auth:/auth:ro"]
    networks: [control, access]
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost:8080/health"]
      interval: 1s
      retries: 20
  cloudflared:
    image: ${GATEWAY_AUTH_IMAGE:-golang:1.26-alpine}
    profiles: [tunnel]
    restart: "no"
    command: ["sleep", "infinity"]
    depends_on:
      gateway-auth:
        condition: service_healthy
  holesail:
    image: ${GATEWAY_AUTH_IMAGE:-golang:1.26-alpine}
    profiles: [tunnel]
    restart: "no"
    command: ["sleep", "infinity"]
    depends_on:
      gateway-auth:
        condition: service_healthy
networks:
  local: {}
  control:
    internal: true
  access: {}
''')
        env = dict(os.environ)
        daemon_host = urllib.parse.urlparse(env.get("DOCKER_HOST", "")).hostname
        remote = daemon_host is not None and daemon_host not in ("localhost", "127.0.0.1", "::1")
        if remote:
            env["GATEWAY_TEST_BIND_ADDR"] = "0.0.0.0"
        context = "gateway-test-" + secrets.token_hex(4)
        base = ["docker", "compose", "-p", "mwcli-mwdd-" + context, "-f", str(work / "compose.yml")]
        binary = Path(os.environ.get("MW_BIN", ROOT / "bin/mw")).resolve()
        if not binary.is_file():
            raise RuntimeError("Build ./bin/mw before running the gateway integration check")
        state = work / "state"
        state.mkdir()
        (state / "config.json").write_text(json.dumps({"telemetry": "no", "timer_last_update_checked": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}))
        cli_env = dict(env, GITLAB_CI="true", CI_PROJECT_DIR=str(state), MWCLI_CONTEXT_TEST="1", PORT="8080", NETWORK_SUBNET_PREFIX="192.168.249", MEDIAWIKI_VOLUMES_CODE=str(work))

        def mw(*args):
            result = subprocess.run([str(binary), "--no-interaction", "dev", "--context", context, *args], text=True, capture_output=True, timeout=60, env=cli_env)
            if result.returncode:
                raise RuntimeError(result.stdout + result.stderr)
            return result.stdout.strip()


        def compose(*args):
            result = subprocess.run(base + list(args), text=True, capture_output=True, timeout=120, env=env)
            if result.returncode:
                raise RuntimeError(result.stdout + result.stderr)
            return result.stdout.strip()

        no_redirect = urllib.request.build_opener(type("NoRedirect", (urllib.request.HTTPRedirectHandler,), {"redirect_request": lambda *args: None})())

        def request(origin, path, headers=None, data=None):
            try:
                response = no_redirect.open(urllib.request.Request(origin + path, headers=headers or {}, data=data), timeout=5)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                return response.status, response.read(), response.headers

        def origin(service):
            published = compose("port", service, "8080")
            port = published.rsplit(":", 1)[1]
            return "http://" + (daemon_host if remote else "127.0.0.1") + ":" + port

        try:
            compose("up", "-d", "traefik", "dashboard")
            local = origin("traefik")
            def wait_local():
                for _ in range(50):
                    try:
                        if request(local, "/proxy/dashboard/")[0] == 200:
                            return
                    except (OSError, urllib.error.URLError):
                        pass
                    time.sleep(0.2)
                raise AssertionError("local proxy did not become ready")

            wait_local()
            assert "gateway-auth" not in compose("ps", "--services", "--filter", "status=running").splitlines()
            print("PASS: local /proxy works without gateway-auth", flush=True)
            compose("up", "-d", "--wait", "gateway-auth")
            tunnel = origin("gateway-auth")
            for path in ["/", "/proxy/dashboard/", "/proxy/adminer/", "/w/api.php"]:
                headers = {"Host": "adminer.local.wmftest.net", "X-Forwarded-Host": "localhost", "X-Forwarded-Uri": "/_wm/login"}
                assert request(tunnel, path, headers)[0] == 401, path
            assert request(tunnel, "/proxy/dashboard/", {"Accept": "text/html"})[0] == 303
            assert request(tunnel, "/_wm/login")[0] == 200
            print("PASS: tunnel rejects unauthenticated and spoofed-host requests; login remains reachable", flush=True)
            basic = "Basic " + base64.b64encode(b"review:review-password").decode()
            code, body, _ = request(tunnel, "/proxy/dashboard/?check=1", {"Authorization": basic, "X-Forwarded-Proto": "https"})
            response = json.loads(body)
            assert code == 200 and response["authorization"] == "" and response["proto"] == "https", response
            assert response["path"] == "/proxy/dashboard/?check=1"
            print("PASS: authenticated ingress preserves paths and HTTPS, stripping gateway credentials", flush=True)
            code, body, headers = request(tunnel, "/_wm/login", {"X-Forwarded-Proto": "https"})
            csrf = re.search(r'name="csrf" value="([^"]+)"', body.decode()).group(1)
            cookies = SimpleCookie()
            for value in headers.get_all("Set-Cookie", []):
                cookies.load(value)
            cookie_header = "; ".join(key + "=" + value.value for key, value in cookies.items())
            form = urllib.parse.urlencode({"username": "review", "password": "review-password", "csrf": csrf}).encode()
            code, _, headers = request(tunnel, "/_wm/login", {"X-Forwarded-Proto": "https", "Cookie": cookie_header, "Content-Type": "application/x-www-form-urlencoded"}, form)
            assert code == 303
            cookies = SimpleCookie()
            for value in headers.get_all("Set-Cookie", []):
                cookies.load(value)
            cookie_header = "; ".join(key + "=" + value.value for key, value in cookies.items() if key.endswith("_session"))
            assert cookie_header
            code, body, _ = request(tunnel, "/proxy/dashboard/", {"X-Forwarded-Proto": "https", "Cookie": cookie_header, "Authorization": "Bearer app-token"})
            assert code == 200 and json.loads(body)["authorization"] == "Bearer app-token"
            assert request(tunnel, "/proxy/dashboard/", {"Cookie": cookie_header})[0] == 401
            print("PASS: browser sessions preserve app tokens and remain bound to their HTTPS origin", flush=True)
            compose("stop", "gateway-auth")
            assert request(local, "/proxy/dashboard/")[0] == 200
            try:
                assert request(tunnel, "/proxy/dashboard/")[0] != 200
            except (OSError, urllib.error.URLError):
                pass
            print("PASS: local access still works after tunnel authentication stops", flush=True)
            # Sleeping fixtures represent providers without establishing exposure.
            compose("up", "-d", "cloudflared", "holesail")
            mw("stop")
            snapshot = state / "mwdd" / context / "state-suspended-services.json"
            saved = json.loads(snapshot.read_text())
            assert set(saved["services"]) == {"dashboard", "traefik"}, saved
            assert not compose("ps", "--services", "--filter", "status=running")
            # Even an old snapshot containing tunnels must not reopen them.
            saved["services"] += ["cloudflared", "holesail", "gateway-auth"]
            snapshot.write_text(json.dumps(saved))
            mw("start")
            local = origin("traefik")
            wait_local()
            assert set(compose("ps", "--services", "--filter", "status=running").splitlines()) == {"dashboard", "traefik"}
            assert request(local, "/proxy/dashboard/")[0] == 200
            print("PASS: dev stop/start restores local access and ignores old saved tunnel entries", flush=True)
            compose("up", "-d", "holesail")
            mw("env", "set", "TUNNEL_PROVIDER", "cloudflare")
            assert "Holesail" in mw("tunnel", "status")
            mw("restart")
            local = origin("traefik")
            wait_local()
            assert set(compose("ps", "--services", "--filter", "status=running").splitlines()) == {"dashboard", "traefik"}
            assert mw("tunnel", "status") == "No tunnel is running."
            assert request(local, "/proxy/dashboard/")[0] == 200
            print("PASS: status detects active Holesail; dev restart closes tunnel access without restoring it", flush=True)

        finally:
            compose("--profile", "*", "down", "--remove-orphans")


if __name__ == "__main__":
    main()
