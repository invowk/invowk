
# Container engine not found!

You tried to use the 'container' runtime but no container engine is available.

## Supported container engines:
- **Podman** (recommended for rootless containers)
- **Docker**

## Things you can try:
- Install Podman:
  - Linux: `sudo apt install podman` or `sudo dnf install podman`
  - macOS: `brew install podman`
  - Windows: Download from https://podman.io

- Install Docker:
  - https://docs.docker.com/get-docker/

- Switch to a different runtime:
~~~cue
default_runtime: "native"  // or "virtual"
~~~

- Configure your preferred engine in ~/.config/invowk/config.cue:
~~~cue
container_engine: "podman"  // or "docker"
~~~