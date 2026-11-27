## Decisions

- **Sync the file, then the directory.** fsync(file) alone gives old-or-new content after power loss (D-Atomic); fsync(dir) after the rename also makes the rename durable, so a returned nil means the new content survives (D-Commit). Both results come from `formal/tla/AtomicWrite.tla` under its named axioms.
- **Windows skips the directory fsync.** Opening a directory handle for fsync is not supported there; NTFS journals metadata operations such as rename.
- **Post-rename failure is reported, not rolled back.** The rename already replaced the target; reverting would need the old content and could itself fail. The error says the content is in place and its durability unknown.

## Risks / Trade-offs

- Two fsyncs add latency to lock-file saves, which are rare and small.
- Filesystems that ignore fsync (for example some network mounts) remain outside the model's axioms.
