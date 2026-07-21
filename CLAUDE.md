# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go library that drives a running VOICEVOX text-to-speech engine: it takes a script — either a
tagged-text string (`[話者][スタイル] テキスト`) or a structured `[]ScriptLine` — resolves each
segment to a speaker/style ID, synthesizes each segment in parallel against the VOICEVOX HTTP API,
combines the resulting WAV files, and writes the result through a caller-supplied `Writer`.
`main.go` is a demo/sample CLI, not the library's real entry point — consumers import
`github.com/shouni/go-voicevox/voicevox` and call `voicevox.New(...)` + `Engine.Run(...)` (text)
or `Engine.RunScript(...)` (structured).

The library has no cloud/storage dependency — output only ever goes through the `Writer` interface
you pass to `voicevox.New(...)`. Use `voicevox.NewLocalWriter()` for local filesystem output, or
implement `contracts.Writer`/`voicevox.Writer` yourself (e.g. to target GCS) if you need something
else; that adapter lives entirely on the caller's side now, not in this repo.

## Commands

```bash
go build ./...
go vet ./...
go test ./...                        # all packages
go test ./internal/engine/...        # single package
go test ./parser -run TestParseXxx   # single test
go run .                             # runs the demo CLI (main.go), needs a live VOICEVOX engine
```

The demo CLI (`main.go`) requires a running VOICEVOX engine reachable at `VOICEVOX_API_URL`
(defaults to `http://localhost:50021`) and writes `output/demo.wav` locally via
`voicevox.NewLocalWriter()` — no cloud credentials needed.

There is no lint config, Makefile, or CI workflow in this repo — `go vet` and `go test` are the
checks to run before considering a change done.

## Architecture

The pipeline is deliberately split into layers, each independently testable behind an interface
defined in `internal/contracts/interfaces.go` (`Engine`, `AudioQueryClient`, `SpeakerClient`,
`DataFinder`, `Parser`, `Writer`):

1. **`voicevox/`** — the public package. `contracts.go` re-exports the `internal/contracts` types
   and options under the `voicevox` name (so callers never import `internal/...` directly).
   `engine.go`'s `voicevox.New(...)` wires everything together: builds the `api.Client`, calls
   `speaker.LoadSpeakers`, picks a `Parser` (plain or phonetic), and constructs the real engine via
   `internalengine.NewWithConfig`. If the caller passes `voicevoxOutput=false`, it returns a
   `noopEngine` instead — a no-op stand-in so callers can disable VOICEVOX without branching their
   own code.

2. **`parser/`** — turns a script string into `[]contracts.Segment`. Lines matching
   `^(\[.+?\])\s*(\[.+?\])\s*(.*)` start a new segment with a `[speaker][style]` tag; untagged
   lines are appended to the current segment (or buffered and attached to the next/previous tagged
   segment with a warning if none exists yet). Segments are force-split at `MaxSegmentCharLength`
   (200 runes) via the exported `parser.SplitByCharLimit(text, limit)`, preferring to break at the
   last `。、！？` within the limit. Inline emotion tags like `[呼びかけ]` are stripped from the final
   text via `reEmotionParse`, not treated as speaker/style tags. `NewPhoneticParser()` wraps this
   with a `github.com/shouni/audio/phonetic` converter that rewrites text to kana readings before
   synthesis (opt-in via `WithPhoneticPreprocessing` / `WithParser`). `SplitByCharLimit` is also
   reused directly by `internal/engine`'s structured `RunScript` path (see below), so both entry
   points share the exact same force-split behavior.

3. **`speaker/`** — resolves tags to VOICEVOX style IDs. `LoadSpeakers` calls `/speakers`, filters
   the response down to `SupportedSpeakers` (`const.go` — currently ずんだもん and 四国めたん only),
   and builds `SpeakerData.StyleIDMap` (`"[めたん][ノーマル]"` → style ID) and `DefaultStyleMap`
   (`"[めたん]"` → its ノーマル tag). Loading fails hard if any supported speaker is missing a
   ノーマル style — that's the required fallback target.

4. **`internal/engine/`** — the actual orchestration, split by concern:
   - `engine.go` — `Engine` struct + `Run()` (text) / `RunScript()` (structured `[]contracts.ScriptLine`),
     both of which call the same three phases below in sequence, differing only in the prepare step.
   - `prepare.go` — `prepareSegments` (text path) parses the script via the injected `Parser`, then
     calls the shared `resolveStyleIDs`. `prepareScriptSegments` (structured path) instead builds a
     `[speaker][style]` tag directly from each `ScriptLine`'s `Speaker`/`Style` fields (no regex),
     force-splits overlong `Text` with `parser.SplitByCharLimit`, then calls the same
     `resolveStyleIDs`. `resolveStyleIDs` looks up each segment's style ID via `style_resolver.go`'s
     `getStyleID` (checks a mutex-guarded cache → `DataFinder.GetStyleID` on the exact tag → falls
     back to `DataFinder.GetDefaultTag(baseSpeakerTag)`'s ノーマル style if the tag doesn't exist).
     If **every** segment fails to resolve, `Run`/`RunScript` aborts before touching the network.
   - `synthesis.go` — `runSynthesisBatch` runs `/audio_query` + `/synthesis` per segment through
     `golang.org/x/sync/errgroup` (`SetLimit(MaxParallelSegments)`) with a shared
     `golang.org/x/time/rate.Limiter` gate and a per-segment `context.WithTimeout`. Segments that
     failed to resolve a style ID are skipped rather than sent to the API. Results are collected
     back into their original order (indexed slice, not append order).
   - `output.go` — `finalizeOutput` combines any pre-calc + runtime errors into a single
     `ErrSynthesisBatch` if there were any, otherwise combines the successful WAV byte slices with
     `github.com/shouni/audio/wav`'s `CombineWavData` and writes via the injected `Writer`.
     (Note: the README describes this as living in `api/audio.go` — that has moved out to the
     external `shouni/audio/wav` package; trust this file over the README diagram on that point.)
   - `errors.go` — `ErrSynthesisBatch` aggregates every segment failure (parse-time and
     runtime) into one error rather than failing on the first one, so a caller can see the full
     picture of what went wrong in a batch.

5. **`api/`** — thin HTTP client for the three VOICEVOX endpoints used
   (`RunAudioQuery` → `/audio_query`, `RunSynthesis` → `/synthesis`, `GetSpeakers` → `/speakers`),
   built on `github.com/shouni/go-http-kit/httpkit.Requester` for retries/error handling. Defines
   its own `ErrAPINetwork` / `ErrInvalidJSON` error types (`errors.go`).

### Key invariants

- Tags passed to `voicevox.WithFallbackTag(...)` (and any tag entering `DataFinder.GetStyleID`)
  must be a **complete** `[speaker][style]` tag, e.g. `"[ずんだもん][ノーマル]"`. A style-only tag
  like `"[ノーマル]"` is invalid on its own.
- `contracts.ScriptLine` (re-exported as `voicevox.ScriptLine`) holds `Speaker`/`Style` **without**
  brackets (e.g. `Speaker: "ずんだもん"`, not `"[ずんだもん]"`) — `prepareScriptSegments` adds the
  brackets when building the internal tag. This is the opposite convention from the text-parser
  path, where tags always carry their brackets end-to-end; don't mix the two up when wiring a new
  caller.
- `Engine.Run` (fallback tag applies to untagged text) and `Engine.RunScript` (every line already
  names its own speaker/style explicitly) are separate entry points for a reason — `RunScript`
  ignores `WithFallbackTag` because there's no "untagged" case in structured input.
- `Engine` in `internal/engine` depends only on the interfaces in `internal/contracts`, not on
  concrete `api.Client` / `speaker.SpeakerData` / `parser.textParser` types — when adding tests or
  alternate implementations, satisfy `AudioQueryClient`/`DataFinder`/`Parser`/`Writer` rather than
  reaching for the concrete structs.
- Output ordering is preserved through the parallel synthesis stage by writing into a
  pre-sized, indexed slice (`results[index] = ...`) rather than appending from goroutines.
- `voicevox/contracts.go` is the seam between the internal engine and public API — new
  configuration options should be added to `internal/contracts` first, then re-exported here.
