# Run a demo pod on Toolforge

Use the embedded kubectl to create a small Python HTTP app, inspect it and open it from your computer. You need a Toolforge account, membership of a tool and working SSH access to Toolforge. The CLI connects through SSH and configures Kubernetes access for the selected tool; a separate kubectl installation or local kubeconfig is not needed.

For routine applications, prefer the [Toolforge jobs framework](https://wikitech.wikimedia.org/wiki/Help:Toolforge/Running_jobs) or webservice tooling. This is an advanced Kubernetes walkthrough: [manually managed resources are not supported by Toolforge administrators](https://wikitech.wikimedia.org/wiki/Help:Toolforge/Kubernetes) and can need changes after platform upgrades.

## Inspect your tool

Run these commands on your computer. Set your tool name, then check what is already running:

```sh
TOOL=your-tool
mw tools kubernetes get pods --tool "$TOOL"
mw tools kubernetes get deployments --tool "$TOOL"
mw tools kubernetes get services --tool "$TOOL"
```

`--tool` selects the tool account and its Kubernetes namespace. `kube`, `k` and `kubectl` are aliases for `kubernetes`. Put the tool flag before `--` when passing a command to a container.

The demo uses one pod named `cli-guide-hello`. It creates no Service or Ingress and does not change your tool's existing webservice. Choose a different demo name if that name is already in use; use the same replacement in the manifest and later commands.

## Create the app

Check the available quota and images first:

```sh
mw tools kubernetes describe resourcequota --tool "$TOOL"
mw tools job images --tool "$TOOL"
```

Save this manifest in your current directory. The image is one of the Toolforge Python images; `job images` lists the currently available images. The app writes a greeting into its own temporary directory and serves it with Python’s built-in HTTP server.

```sh
cat > cli-guide-hello.yaml <<'YAML'
apiVersion: v1
kind: Pod
metadata:
  name: cli-guide-hello
  labels:
    toolforge: tool
spec:
  restartPolicy: Never
  activeDeadlineSeconds: 3600
  automountServiceAccountToken: false
  containers:
    - name: hello
      image: docker-registry.svc.toolforge.org/toolforge-python313-sssd-web:latest
      command: [sh, -c]
      args:
        - |
          mkdir -p /tmp/cli-guide-hello
          printf 'Hello from Toolforge!\n' > /tmp/cli-guide-hello/index.html
          exec python3 -u -m http.server 8000 --directory /tmp/cli-guide-hello
      resources:
        requests: {cpu: 10m, memory: 32Mi}
        limits: {cpu: 100m, memory: 128Mi}
      readinessProbe:
        httpGet: {path: /, port: 8000}
YAML
```

Check that the cluster accepts it, then create the pod and wait for readiness:

```sh
mw tools kubernetes create --dry-run=server -f cli-guide-hello.yaml --tool "$TOOL"
mw tools kubernetes create -f cli-guide-hello.yaml --tool "$TOOL"
mw tools kubernetes wait --for=condition=Ready pod/cli-guide-hello --timeout=120s --tool "$TOOL"
```

`create` fails if that name already exists instead of updating another pod. The resource requests and limits keep the demo small. It stops automatically after an hour; delete it when finished to release the pod quota too.

## Inspect the pod

```sh
mw tools kubernetes get pod cli-guide-hello --tool "$TOOL"
mw tools kubernetes logs cli-guide-hello --tool "$TOOL"
mw tools kubernetes describe pod cli-guide-hello --tool "$TOOL"
mw tools kubernetes exec cli-guide-hello --tool "$TOOL" -- python3 --version
```

The logs should include `Serving HTTP on` and port 8000. If readiness times out, look at the Events section of `describe` for image, quota or scheduling errors.

## Open the app

In one terminal, forward a localhost port to the demo's HTTP port:

```sh
mw tools kubernetes port-forward pod/cli-guide-hello 18080:8000 --tool "$TOOL"
```

Leave that command running and open <http://127.0.0.1:18080/> in your browser, or use another terminal:

```sh
curl --fail http://127.0.0.1:18080/
```

You should see `Hello from Toolforge!`. Port forwarding binds to localhost by default and lasts until you press Ctrl-C. If port 18080 is busy, choose another local port on the left of `:`. This does not publish the demo on your tool's public domain.

## Clean up

Stop port forwarding with Ctrl-C, then delete only the demo pod:

```sh
mw tools kubernetes delete pod cli-guide-hello --tool "$TOOL"
mw tools kubernetes get pods --tool "$TOOL"
rm cli-guide-hello.yaml
```

A bare pod is useful for this short experiment but is not recreated after deletion or a node failure. For a long-running application, use the Toolforge jobs framework or learn about [Kubernetes Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).
