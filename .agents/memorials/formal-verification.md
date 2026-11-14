# Formal verification history

Dated observations moved out of the rules on 2026-11-14. None of this is normative.

- **TLA+ 1.8.0** was still a prerelease as of 2026-09-24, so `formal/manifest.toml` pins `1.7.4` (prerelease builds are never pinned).
- **First formal-bindings mutation run** took 28 min on one local worker, including a ~90 s pre-flight (results in `formal/README.md`). At that time the plan used two `--match` groups: `ScriptFilePath.Validate` ran apart from the files that declare other `Validate` methods.
