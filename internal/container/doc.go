// SPDX-License-Identifier: MPL-2.0

// Package container provides a unified abstraction layer for container engines (Docker/Podman).
//
// The Engine interface defines the core operations: Build, Run, Remove, ImageExists, and RemoveImage.
// Two implementations are provided: DockerEngine and PodmanEngine, both embedding BaseCLIEngine
// for shared CLI argument construction and command execution.
//
// Engine selection uses NewEngine(EngineType) with automatic fallback if the preferred engine
// is unavailable, or AutoDetectEngine() for preference-less detection (Podman is tried first).
//
// IMPORTANT: Only Linux containers are supported. Alpine-based images are not supported due to
// musl compatibility issues, and Windows container images are not supported. Use debian:stable-slim
// as the reference container image in tests and examples.
package container
