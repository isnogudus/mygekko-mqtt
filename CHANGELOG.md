# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Command queue: incoming MQTT set commands are now decoupled from the MyGEKKO
  HTTP request via a buffered queue drained by a dedicated worker, so the MQTT
  receive loop is never blocked by a slow command.
- `mygekko.command_interval` (default: 20.0s): minimum gap between throttled set
  commands. MyGEKKO is single-threaded and drops/crashes on commands that arrive
  too quickly after one another, so throttled commands are serialized and spaced.
- `mygekko.throttle_prefixes`: per-category partition into throttled and
  immediate commands. For a listed category, only payloads starting with one of
  the given prefixes are throttled (e.g. blinds `P50`); every other command
  (e.g. a STOP) is sent immediately and preempts an active throttle wait.
- Debug log line `Command ok` after a successful set command.

### Changed
- A failed `SetValue` command or an invalid set topic is now logged and skipped
  instead of terminating the bridge (former exit codes 9 and 8). A single bad
  command no longer drops the other commands still queued behind it.
- `SetValue` now treats an empty body and an empty JSON object `{}` as success,
  in addition to the literal `OK` — MyGEKKO answers some set endpoints with `{}`.

### Fixed
- Bursts of set commands losing all but the first command: MyGEKKO replied to a
  blind position command with HTTP 200 and body `{}`, which was treated as an
  error and crashed the daemon (exit 9), discarding the queued commands.
