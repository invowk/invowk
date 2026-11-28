# Spec Delta

## ADDED Requirements

### Requirement: The viewer reports its own session status via OSC 7501
While isolating child terminal queries and control sequences inside the emulated terminal screen, the `-i` viewer SHALL emit its own OSC 7501 `ProgramStatus` reports (`working`, `blocked`, `idle`, `done`, `error`, and `clear` on dismiss/cancel) to the outer terminal via Bubble Tea's view state.

#### Scenario: Viewer emits outer status while isolating child OSC output
- **GIVEN** an interactive `-i` command runs inside the viewer
- **WHEN** the command executes, triggers a TUI overlay, completes, and is dismissed
- **THEN** the outer terminal SHALL receive only the viewer's own OSC 7501 transitions, while any raw OSC status/query or control sequences written by the child (other than rendered OSC 8 hyperlinks) remain confined to the emulated screen
