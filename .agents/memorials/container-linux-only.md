# Why the container runtime is Linux-only (recorded 2026-11-14)

The policy itself lives in `AGENTS.md` ("Hard invariants"). This note keeps the reasons.

- **No Alpine images.** musl-based environments have many subtle gotchas. The project prioritizes reliability over image size, so the reference image is `debian:stable-slim`.
- **No Windows container images.** They are rarely used, and supporting them would add too much complexity to auto-provisioning, which attaches an ephemeral image layer holding the `invowk` binary and the needed invowkfiles and invowkmods to the user's image or containerfile.
