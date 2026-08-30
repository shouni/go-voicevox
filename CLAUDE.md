# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go library that drives a running VOICEVOX text-to-speech engine: it takes a structured
`[]ScriptLine` script, resolves each line's speaker/style to a VOICEVOX style ID, converts each
segment's text to a katakana reading (to avoid VOICEVOX mispronouncing kanji), synthesizes each
segment in parallel against the VOICEVOX HTTP API, combines the resulting WAV files, and returns
the combined WAV bytes. `cmd/voicevox-demo` is a demo/sample CLI, not the library's real entry
point — consumers import `github.com/shouni/go-voicevox/voicevox` and call `voicevox.New(...)` +
`Engine.Run(ctx, lines)`.

The library's responsibility ends at "structured script in, WAV bytes out." It has no I/O
dependency beyond the VOICEVOX HTTP API itself — it does not write files, does not know about
cloud storage, and does not accept a `Writer` of any kind. Saving the returned bytes (to a local
file, to GCS, wherever) is entirely the caller's job; `cmd/voicevox-demo` demonstrates this with
a plain `os.WriteFile` call.

## Commands

```bash
go build ./...
go vet ./...
go test ./...                        # all packages
go test ./internal/engine/...        # single package
go run ./cmd/voicevox-demo           # runs the demo CLI, needs a live VOICEVOX engine
```

The demo CLI (`cmd/voicevox-demo`) requires a running VOICEVOX engine reachable at
`VOICEVOX_API_URL` (defaults to `http://localhost:50021`) and writes `output/demo.wav` locally via
a plain `os.WriteFile` call on the bytes `Engine.Run` returns — no cloud credentials needed. **The
demo command has no test file and should not get one**: running the demo against a live engine and
playing the WAV back *is* its test, and a unit test over its constants only restates them.

There is no Makefile. `.github/workflows/ci.yml` runs on pushes and PRs to `main`/`develop` in
three jobs: build + `go vet` + `gofmt -l` + `go test -race`, then `golangci-lint` (config in
`.golangci.yml`), then `govulncheck` — run those four locally before considering a change done.

## Architecture

The pipeline is deliberately split into layers, each independently testable behind an interface
defined **in the package that consumes it** rather than in a shared bag of types:
`internal/engine` declares `AudioQueryClient` / `StyleFinder` / `TextConverter`, `speaker`
declares `Client`, and `voicevox` declares `Engine`. There is no `contracts`/`types`/`models`
package — none of the sibling libraries has one either, and a package named for the *kind* of
thing inside says nothing about what it provides:

1. **`voicevox/`** — the public package. `exports.go` re-exports only what the public API can
   actually be used with — `Engine`, `ScriptLine`, `Option` — so callers never import
   `internal/...` directly. **The internal seams are not re-exported.** `New` builds its own
   client and speaker data, so there is no way to supply an `AudioQueryClient` or a `StyleFinder`
   through the public API; listing them would advertise a substitution that cannot be made.
   The internal `segment` type is unexported for the same reason: its tags are assembled by
   `prepareSegments`, and the way in is `ScriptLine`. **The `Default*` constants are not
   re-exported either** — a caller that wants the default omits the option, so `WithX(DefaultX)`
   was only ever a call that did nothing; the values are stated in each `WithX` doc comment
   instead, and the one consumer (ap-voice) keeps its own different defaults in its config.
   `options.go` holds the public `Option` type and every `WithX`. **`Option` is *not* an alias of
   `internalengine.Option`** — it is `func(*options)`, where `options` splits what it was given by
   destination: `engine []internalengine.Option` and `converter []phonetic.Option`. The split
   exists because the settings have two readers: parallelism/timeout/rate belong to the synthesis
   engine, while reading overrides belong to the converter `New` builds. Keeping the alias would
   have meant putting a `ReadingOverrides` field on `engine.Config` that `internal/engine` never
   reads — the exact dead-weight shape this repo keeps deleting.
   `engine.go`'s `voicevox.New(...)` wires everything together: builds the `api.Client`, calls
   `registry.LoadStyles`, constructs a `github.com/shouni/audio/phonetic.Converter` (the reading
   converter — this is not optional, every `Run` call goes through it, though
   `WithReadingOverrides` feeds it a per-application reading vocabulary), and constructs
   the real engine via `internalengine.New`, forwarding `o.engine`. **There is no switch for turning synthesis
   off.** A `voicevoxOutput bool` used to select a no-op `Engine`; the only caller wrote the
   constant `true`, so the disabled path never ran. A caller that wants no synthesis can decline
   to call `New`.

   **`WithReadingOverrides` exists for counters and proper nouns.** The converter passes digits
   through untouched and VOICEVOX reads them literally, so `8日` becomes ハチニチ (not ヨウカ),
   `1人` イチニン, `20歳` ニジュッサイ. Nothing downstream reveals this — the converted text still
   shows the digit — so it is invisible until synthesis. The overrides layer on top of
   `phonetic`'s bundled defaults; an empty/nil map is ignored rather than forwarded, so a caller
   passing `nil` does not make the converter rebuild its key index for nothing. Which words get
   which reading is application vocabulary, for the same reason the speaker roster is: it differs
   per work, and baking it in would mean a library release per word.

2. **`speaker/`** — resolves tags to VOICEVOX style IDs, and **holds the structure of the
   `/speakers` response but none of its data**. It declares `Client` (the one-method interface
   `LoadStyles` needs) itself rather than importing an internal one: a public signature naming
   an `internal/` type is one a caller can satisfy but cannot write down. `Registry` (`registry.go`) is built by the
   caller from a saved `/speakers` payload (`speaker.NewRegistry(raw)`); which speakers an app
   uses is application policy, not an engine concern, so baking a roster into the library would
   mean cutting a release to add one speaker and would stop two apps from casting differently.
   `NewRegistry` normalizes as it validates: it drops the non-talk styles once and stores the
   result as `speakerEntry{name, styles}` plus an `indexByName` index, so lookups are a map hit
   and nothing re-filters. It used to keep the raw payload and call `talkStyles()` again on every
   question — `LoadStyles` asks two per engine speaker, so the filtering ran over and over on a
   list that cannot change after construction. A duplicate name still resolves to the first
   occurrence, as the old linear scan did.
   `Registry` exposes `SpeakerNames()` / `StyleNames()` / `StylesFor(name)` /
   `DefaultStyleFor(name)` for callers that must enumerate the vocabulary offline — e.g. building
   a Gemini `ResponseSchema` enum without a network call. **`StylesFor` is the one to reach for**:
   `StyleNames()` is the union across speakers, and offering a combination that does not exist is
   not an error — `getStyleID` quietly falls back to that speaker's default — so a schema built
   from the union asks for something the output silently ignores.

   `(*Registry).LoadStyles(ctx, client)` calls `/speakers` and builds the `Styles` map
   (`"[四国めたん][ノーマル]"` → style ID) plus a per-speaker default tag. **It is a method on
   `Registry`, not a free function**, because the roster *is* the filter: as
   `LoadSpeakers(ctx, client, allowed)` the subject sat in the third argument and nothing at the
   call site explained what passing `nil` meant. **A nil receiver is legal and means "no
   filter"** — `voicevox.New` forwards a caller's omitted roster straight through, so that path is
   pinned by a test. `Styles`' maps are unexported: only `LoadStyles` assembles them, and nothing
   should be able to rewrite a style ID mid-synthesis. **Speaker names are the VOICEVOX
   spelling, not short tags** (`四国めたん`, not `めたん`). **Style IDs always come from the live
   engine**, never from a saved payload, because they shift between engine builds and a stale one
   speaks in the wrong character's voice. The **default style is each speaker's first talk
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
     `splitByCharLimit` (`textsplit.go`), converts each resulting chunk to a katakana reading via
     `Engine.converter` (`TextConverter`, backed by
     `github.com/shouni/audio/phonetic.Converter` in production — split-then-convert, matching the
     original tagged-text parser's ordering, so the 200-rune limit is measured on the pre-conversion
     text), and resolves each chunk's style ID in the same pass via `style_resolver.go`'s
     `styleResolver.resolve` (checks a cache → `StyleFinder.GetStyleID` on the exact tag →
     falls back to `StyleFinder.GetDefaultTag`'s default style if the tag doesn't exist).
     **The resolver — cache included — lives for one `Run`**, built at the top of
     `prepareSegments`. The cache used to sit on `Engine` behind an `RWMutex`; since what it wraps
     is itself a map lookup, carrying it across `Run` calls bought almost nothing while making
     "can two goroutines `Run` the same `Engine`?" a question about that lock. Scoped to one batch,
     `Engine` is immutable after construction and the answer is unconditionally yes
     (`concurrent_test.go` pins it under `-race`). A per-batch cache also means the fallback
     warning is logged once *per batch* rather than once per process, so the second job no longer
     silently repeats the first job's miss. If
     **every** segment fails to resolve, `Run` aborts before touching the network.
     It also declares `segment`, the one internal unit. There used to be two — a tag-and-text
     `Segment` converted into an `engineSegment` carrying `StyleID`/`Err` by a second pass — but
     the first was never handed anywhere on its own, so the split bought a type and a loop and
     nothing else.
   - `textsplit.go` — `splitByCharLimit` / `maxSegmentCharLength` (200 runes): splits overlong
     segment text at the **last** `。、！？` within the limit — cutting at the first one would
     leave a trail of short fragments and an uneven cadence — falling back to a mechanical cut if
     no punctuation is found. `cutHead` always returns at least one rune when it splits, so the
     loop cannot stall and needs no guard against it. **Unexported**, since `prepareSegments` is
     its only caller and no caller chooses the limit.
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
     whether the rate limit or the parallelism is the binding constraint. Per-segment progress
     attributes go through `slog.Group`, not a `map[string]any` — a map reaches the handler as a
     single opaque value, so it printed as a Go struct literal in `TextHandler` and lost its
     per-key types in JSON; as a group it expands to `current_segment.index` and friends.
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
   separate status error type. **`RunAudioQuery` does not decode the response** — it checks
   `json.Valid` and returns the bytes as they came, because they go straight to `/synthesis`; it
   used to unmarshal into an `AudioQueryResponse` whose fields nothing ever read, which only made
   the layer look like it interpreted the payload. **It is internal**: nothing outside the module
   imported it, and `LoadStyles` is the only public entry point that would need one — a caller can
   satisfy `speaker.Client` with one method. Because these error types are now unnameable from outside,
   **no public function may return one**; `speaker` has its own `ErrInvalidPayload` for that.

### Key invariants

- `ScriptLine` (re-exported as `voicevox.ScriptLine`) holds `Speaker`/`Style` **without**
  brackets (e.g. `Speaker: "ずんだもん"`, not `"[ずんだもん]"`) — `prepareSegments` adds the brackets
  when building the internal tag.
- **`ScriptLine` carries only what gets synthesized** — speaker, style, text. It once had a
  `Direction` field for downstream video cues, justified as letting callers round-trip their own
  domain data; nothing ever read it, and the one consumer dropped it from its own model, so the
  field cost tokens in every AI response for no reader.
- **`Engine` holds no mutable state.** Every field is set in `New` and read-only afterwards
  (`rate.Limiter` synchronizes itself), which is what makes concurrent `Run` on one `Engine` safe.
  Anything a batch needs to accumulate belongs to that batch, not to `Engine`.
- `Engine` in `internal/engine` depends only on the interfaces it declares, not on
  concrete `api.Client` / `speaker.Styles` types — when adding tests or alternate
  implementations, satisfy `AudioQueryClient`/`StyleFinder` rather than reaching for the concrete
  structs.
- **One constructor per thing.** `internal/engine` carried both `New(..., opts ...Option)` and
  `NewWithConfig(..., cfg Config)`; production went through one and every test through the other,
  so neither door was exercised by the other's callers. `New` takes options and builds the config
  itself; anything wanting a prepared `Config` can pass it as an option.
- Output ordering is preserved through the parallel synthesis stage by writing into a
  pre-sized, indexed slice (`results[index] = ...`) rather than appending from goroutines.
- `voicevox/exports.go` is the seam between the internal engine and public API; it holds only
  what a caller can actually touch (`Engine`, `ScriptLine`). New configuration goes to
  `voicevox/options.go`, and to `internal/engine/options.go` as well **only when the synthesis
  engine is what reads it** — a `WithX` whose value is consumed at wiring time (like
  `WithReadingOverrides`, read by the converter) stops at the public package and never becomes an
  `engine.Config` field. The alias direction is forced: `internal/engine` cannot import `voicevox`
  (it would cycle), so any type both sides need must live in `internal/engine`. `Engine` is the
  exception — only the public package uses that interface, so it is declared there outright.
- Before adding an `Option`/config field, verify it's actually reachable and read somewhere in
  `internal/engine` — a config knob that nothing ever consumes is dead weight, not a feature (this
  is exactly why the old `WriteOption`/`Writer` plumbing and the tagged-text parsing path were
  removed: they existed on paper but had no real caller or no real configuration path).
