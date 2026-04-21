# Running the workshop on Podman

If you have Podman installed and prefer to use it over Docker, the workshop runs unchanged — Podman speaks the Docker API through its compatibility socket, and the `docker compose` CLI drives it end-to-end.

## Set `DOCKER_HOST`

Point `docker compose` at Podman's socket:

```bash
# macOS (Podman Machine)
export DOCKER_HOST="unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')"

# Linux rootless
export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"

# Linux rootful
export DOCKER_HOST="unix:///run/podman/podman.sock"
```

```powershell
# Windows (PowerShell)
$env:DOCKER_HOST = "npipe:////./pipe/podman-machine-default"
```

Confirm: `docker ps` should succeed.

## macOS / Windows: let containers read the Docker socket

With Podman Machine running, run:

```bash
podman machine ssh sudo setenforce 0
```

This SSHes into the VM and relaxes its default security policy so the collector and OBI can bind-mount the Docker socket — otherwise they exit on startup. The setting resets on each machine boot, so re-run it after any `podman machine start` or `podman machine set`.

## Rootful mode for Exercise 02

[Exercise 02](exercises/02-setup-obi.md)'s OBI loads eBPF programs, which need privileged kernel access (`CAP_SYS_ADMIN`, `CAP_BPF`, `CAP_PERFMON`, and a few others — see [OBI security](https://opentelemetry.io/docs/zero-code/obi/security/)). Rootless Podman can't grant those, so you need rootful mode for this exercise.

**macOS / Windows:** Podman Machine defaults to rootless inside the VM. Switch the machine to rootful once:

```bash
podman machine stop
podman machine set --rootful
podman machine start
```

Re-run the `DOCKER_HOST` assignment from the previous section — it now resolves to the rootful socket automatically.

**Linux:** Podman runs containers directly against your host kernel (no VM), so the caps have to come from the host. Enable the rootful socket and point `DOCKER_HOST` at it:

```bash
sudo systemctl enable --now podman.socket
export DOCKER_HOST="unix:///run/podman/podman.sock"
```

---

If something breaks, see [TROUBLESHOOTING.md § Podman](TROUBLESHOOTING.md#podman).
