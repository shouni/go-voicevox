# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go library that drives a running VOICEVOX text-to-speech engine: it takes a structured
`[]ScriptLine` script, resolves each line's speaker/style to a VOICEVOX style ID, converts each
segment's text to a katakana reading (to avoid VOICEVOX mispronouncing kanji), synthesizes each
segment in parallel against the VOICEVOX HTTP API, combines the resulting WAV files, and returns
the combined WAV bytes. `main.go` is a demo/sample CLI, not the library's real entry point —
consumers import `github.com/shouni/go-voicevox/voicevox` and call `voicevox.New(...)` +
`Engine.Run(ctx, lines)`.

The library's responsibility ends at "structured script in, WAV bytes out." It has no I/O
dependency beyond the VOICEVOX HTTP API itself — it does not write files, does not know about
cloud storage, and does not accept a `Writer` of any kind. Saving the returned bytes (to a local
file, to GCS, wherever) is entirely the caller's job; `main.go` demonstrates this with a plain
`os.WriteFile` call.

## Commands

```bash
go build ./...
go vet ./...
go test ./...                        # all packages
go test ./internal/engine/...        # single package
go run .                             # runs the demo CLI (main.go), needs a live VOICEVOX engine
```

The demo CLI (`main.go`) requires a running VOICEVOX engine reachable at `VOICEVOX_API_URL`
(defaults to `http://localhost:50021`) and writes `output/demo.wav` locally via a plain
`os.WriteFile` call on the bytes `Engine.Run` returns — no cloud credentials needed.

There is no lint config, Makefile, or CI workflow in this repo — `go vet` and `go test` are the
checks to run before considering a change done.

## Architecture

The pipeline is deliberately split into layers, each independently testable behind an interface
defined in `internal/contracts/interfaces.go` (`Engine`, `AudioQueryClient`, `SpeakerClient`,
`DataFinder`, `TextConverter`):

1. **`voicevox/`** — the public package. `contracts.go` re-exports the `internal/contracts` types
   and options under the `voicevox` name (so callers never import `internal/...` directly).
   `engine.go`'s `voicevox.New(...)` wires everything together: builds the `api.Client`, calls
   `speaker.LoadSpeakers`, constructs a `github.com/shouni/audio/phonetic.Converter` (the reading
   converter — this is not optional/configurable, every `Run` call goes through it), and constructs
   the real engine via `internalengine.NewWithConfig`. If the caller passes `voicevoxOutput=false`,
   it returns a `noopEngine` instead — a no-op stand-in so callers can disable VOICEVOX without
   branching their own code (its `Run` returns `nil, nil`).

2. **`speaker/`** — resolves tags to VOICEVOX style IDs. `LoadSpeakers` calls `/speakers`, filters
   the response down to `SupportedSpeakers` (`const.go` — currently ずんだもん and 四国めたん only),
   and builds `SpeakerData.StyleIDMap` (`"[めたん][ノーマル]"` → style ID) and `DefaultStyleMap`
   (`"[めたん]"` → its ノーマル tag). Loading fails hard if any supported speaker is missing a
   ノーマル style — that's the required fallback target. `SupportedSpeakerNames()` /
   `SupportedStyleNames()` expose the static supported vocabulary (no network call needed) so a
   caller building an AI response schema (e.g. a Gemini `ResponseSchema` enum) doesn't have to
   hand-duplicate these lists.

3. **`internal/engine/`** — the actual orchestration, split by concern:
   - `engine.go` — `Engine` struct + `Run(ctx, lines) ([]byte, error)`, which calls the three
     phases below in sequence and returns the combined WAV bytes (or an error). It performs no I/O
     beyond the VOICEVOX HTTP calls.
   - `prepare.go` — `prepareSegments` builds a `[speaker][style]` tag directly from each
     `ScriptLine`'s `Speaker`/`Style` fields, force-splits overlong `Text` with the package-local
     `SplitByCharLimit` (`textsplit.go`), converts each resulting chunk to a katakana reading via
     `Engine.converter` (`contracts.TextConverter`, backed by
     `github.com/shouni/audio/phonetic.Converter` in production — split-then-convert, matching the
     original tagged-text parser's ordering, so the 200-rune limit is measured on the pre-conversion
     text), then calls `resolveStyleIDs`. `resolveStyleIDs` looks up each segment's style ID via
     `style_resolver.go`'s `getStyleID` (checks a mutex-guarded cache → `DataFinder.GetStyleID` on
     the exact tag → falls back to `DataFinder.GetDefaultTag`'s ノーマル style if the tag doesn't
     exist). If **every** segment fails to resolve, `Run` aborts before touching the network.
   - `textsplit.go` — `SplitByCharLimit` / `MaxSegmentCharLength` (200 runes): splits overlong
     segment text at the last `。、！？` within the limit, falling back to a mechanical cut if no
     punctuation is found. Package-local to `internal/engine` since `prepareSegments` is its only
     caller.
   - `synthesis.go` — `runSynthesisBatch` runs `/audio_query` + `/synthesis` per segment through
     `golang.org/x/sync/errgroup` (`SetLimit(MaxParallelSegments)`) with a shared
     `golang.org/x/time/rate.Limiter` gate and a per-segment `context.WithTimeout`. Segments that
     failed to resolve a style ID are skipped rather than sent to the API. Results are collected
     back into their original order (indexed slice, not append order).
   - `output.go` — `combineOutput` combines any pre-calc + runtime errors into a single
     `ErrSynthesisBatch` if there were any, otherwise combines the successful WAV byte slices with
     `github.com/shouni/audio/wav`'s `CombineWavData` and returns the result. It does not write
     anywhere — the caller decides what to do with the bytes.
   - `errors.go` — `ErrSynthesisBatch` aggregates every segment failure (parse-time and
     runtime) into one error rather than failing on the first one, so a caller can see the full
     picture of what went wrong in a batch.

4. **`api/`** — thin HTTP client for the three VOICEVOX endpoints used
   (`RunAudioQuery` → `/audio_query`, `RunSynthesis` → `/synthesis`, `GetSpeakers` → `/speakers`),
   built on `github.com/shouni/go-http-kit/httpkit.Requester` for retries/error handling. Defines
   its own `ErrAPINetwork` / `ErrInvalidJSON` error types (`errors.go`).

### Key invariants

- `contracts.ScriptLine` (re-exported as `voicevox.ScriptLine`) holds `Speaker`/`Style` **without**
  brackets (e.g. `Speaker: "ずんだもん"`, not `"[ずんだもん]"`) — `prepareSegments` adds the brackets
  when building the internal tag.
- `ScriptLine.Direction` is an optional caller-side annotation (e.g. for downstream video
  direction/emotion cues). The engine never reads it — it exists purely so callers can round-trip
  their own domain data through `ScriptLine` without a separate parallel struct.
- `Engine` in `internal/engine` depends only on the interfaces in `internal/contracts`, not on
  concrete `api.Client` / `speaker.SpeakerData` types — when adding tests or alternate
  implementations, satisfy `AudioQueryClient`/`DataFinder` rather than reaching for the concrete
  structs.
- Output ordering is preserved through the parallel synthesis stage by writing into a
  pre-sized, indexed slice (`results[index] = ...`) rather than appending from goroutines.
- `voicevox/contracts.go` is the seam between the internal engine and public API — new
  configuration options should be added to `internal/contracts` first, then re-exported here.
- Before adding an `Option`/config field, verify it's actually reachable and read somewhere in
  `internal/engine` — a config knob that nothing ever consumes is dead weight, not a feature (this
  is exactly why the old `WriteOption`/`Writer` plumbing and the tagged-text parsing path were
  removed: they existed on paper but had no real caller or no real configuration path).
