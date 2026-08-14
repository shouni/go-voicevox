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

1. **`voicevox/`** — the public package. `contracts.go` re-exports only what the public API can
   actually be used with — `Engine`, `ScriptLine`, `Option` — so callers never import
   `internal/...` directly. **The internal seams are not re-exported.** `New` builds its own
   client and speaker data, so there is no way to supply an `AudioQueryClient` or a `DataFinder`
   through the public API; listing them would advertise a substitution that cannot be made.
   `Segment` is internal for the same reason: its tags are assembled by `prepareSegments`, and
   the way in is `ScriptLine`.
   `engine.go`'s `voicevox.New(...)` wires everything together: builds the `api.Client`, calls
   `speaker.LoadSpeakers`, constructs a `github.com/shouni/audio/phonetic.Converter` (the reading
   converter — this is not optional/configurable, every `Run` call goes through it), and constructs
   the real engine via `internalengine.NewWithConfig`. **There is no switch for turning synthesis
   off.** A `voicevoxOutput bool` used to select a no-op `Engine`; the only caller wrote the
   constant `true`, so the disabled path never ran. A caller that wants no synthesis can decline
   to call `New`.

2. **`speaker/`** — resolves tags to VOICEVOX style IDs, and **holds the structure of the
   `/speakers` response but none of its data**. It declares `Client` (the one-method interface
   `LoadSpeakers` needs) itself rather than importing an internal one: a public signature naming
   an `internal/` type is one a caller can satisfy but cannot write down. `Registry` (`registry.go`) is built by the
   caller from a saved `/speakers` payload (`speaker.NewRegistry(raw)`); which speakers an app
   uses is application policy, not an engine concern, so baking a roster into the library would
   mean cutting a release to add one speaker and would stop two apps from casting differently.
   `Registry` exposes `SpeakerNames()` / `StyleNames()` / `StylesFor(name)` /
   `DefaultStyleFor(name)` for callers that must enumerate the vocabulary offline — e.g. building
   a Gemini `ResponseSchema` enum without a network call. **`StylesFor` is the one to reach for**:
   `StyleNames()` is the union across speakers, and offering a combination that does not exist is
   not an error — `getStyleID` quietly falls back to that speaker's default — so a schema built
   from the union asks for something the output silently ignores.

   `LoadSpeakers(ctx, client, allowed)` calls `/speakers` and builds `SpeakerData.StyleIDMap`
   (`"[四国めたん][ノーマル]"` → style ID) and `DefaultStyleMap`. **Speaker names are the VOICEVOX
   spelling, not short tags** (`四国めたん`, not `めたん`). **Style IDs always come from the live
   engine**, never from a saved payload, because they shift between engine builds and a stale one
   speaks in the wrong character's voice. Passing `allowed == nil` accepts everything the engine
   offers; a non-nil `Registry` narrows it. The **default style is each speaker's first talk
   style, not ノーマル** — 白上虎太郎 (ふつう), 後鬼 (人間ver.) and 里石ユカ (つぼみ) have no ノーマル
   at all. Non-talk styles (singing) are skipped since `/synthesis` cannot use them. Loading fails
   only when *no* speaker could be assembled: the saved roster being newer than the engine is
   normal, so a per-speaker mismatch degrades to the styles that did resolve rather than refusing
   to start.

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
     segment text at the **last** `。、！？` within the limit — cutting at the first one would
     leave a trail of short fragments and an uneven cadence — falling back to a mechanical cut if
     no punctuation is found. `cutHead` always returns at least one rune when it splits, so the
     loop cannot stall and needs no guard against it. Package-local to `internal/engine` since
     `prepareSegments` is its only caller.
   - `synthesis.go` — `runSynthesisBatch` runs `/audio_query` + `/synthesis` per segment through
     `golang.org/x/sync/errgroup` (`SetLimit(MaxParallelSegments)`) with a shared
     `golang.org/x/time/rate.Limiter` gate and a per-segment `context.WithTimeout`. Segments that
     failed to resolve a style ID are skipped rather than sent to the API. Results are collected
     back into their original order (indexed slice, not append order) — and the returned slice
     **keeps a nil at every position that was not synthesized**, because `output.go` maps a
     combine error's position back to a segment number through it.
     **Every scheduled segment must leave a result behind.** A goroutine that gives up before
     `processSegment` — the rate limiter returning on a cancelled context — records the failure
     instead of returning early, and `collectSynthesisResults` counts a missing result as an
     error rather than skipping it. Both halves matter: without them a cancelled batch returned
     the segments that happened to finish, with a nil error, so a timed-out job produced a
     truncated WAV and a success notification (`cancel_test.go`).
     No goroutine returns an error, so the group is a plain `errgroup.Group`: aborting the batch
     on the first failure would contradict `ErrSynthesisBatch`, whose point is to report all of
     them. The batch also logs per-segment durations (avg/min/max), which is what tells you
     whether the rate limit or the parallelism is the binding constraint.
   - `output.go` — `combineOutput` combines any pre-calc + runtime errors into a single
     `ErrSynthesisBatch` if there were any, otherwise combines the successful WAV byte slices with
     `github.com/shouni/audio/wav`'s `CombineWavData` and returns the result. It does not write
     anywhere — the caller decides what to do with the bytes.
   - `errors.go` — `ErrSynthesisBatch` aggregates every segment failure (parse-time and
     runtime) into one error rather than failing on the first one, so a caller can see the full
     picture of what went wrong in a batch. It holds `[]error` and implements
     `Unwrap() []error`, so **`errors.Is` / `errors.As` reach through it** — it used to flatten
     everything to `[]string`, which left the caller comparing message text to tell a
     cancellation from an unreachable engine. Note `rate.Limiter` reports a predicted deadline
     overrun with its own error rather than wrapping `ctx.Err()`, so the batch substitutes the
     context's cause when the context is already done.

4. **`internal/api/`** — thin HTTP client for the three VOICEVOX endpoints used
   (`RunAudioQuery` → `/audio_query`, `RunSynthesis` → `/synthesis`, `GetSpeakers` → `/speakers`),
   built on `github.com/shouni/go-http-kit/httpkit.Requester` for retries/error handling. Defines
   its own `ErrAPINetwork` / `ErrInvalidJSON` error types (`errors.go`). Status-code handling and
   retries belong to go-http-kit, so this layer sees only the final outcome — there is no
   separate status error type. **It is internal**: nothing outside the module imported it, and
   `LoadSpeakers` is the only public function that would need one — a caller can satisfy
   `speaker.Client` with one method. Because these error types are now unnameable from outside,
   **no public function may return one**; `speaker` has its own `ErrInvalidPayload` for that.

### Key invariants

- `contracts.ScriptLine` (re-exported as `voicevox.ScriptLine`) holds `Speaker`/`Style` **without**
  brackets (e.g. `Speaker: "ずんだもん"`, not `"[ずんだもん]"`) — `prepareSegments` adds the brackets
  when building the internal tag.
- **`ScriptLine` carries only what gets synthesized** — speaker, style, text. It once had a
  `Direction` field for downstream video cues, justified as letting callers round-trip their own
  domain data; nothing ever read it, and the one consumer dropped it from its own model, so the
  field cost tokens in every AI response for no reader.
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
