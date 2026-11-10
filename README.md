# ✍️ Go VOICEVOX

[![CI](https://github.com/shouni/go-voicevox/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/go-voicevox/actions/workflows/ci.yml)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-voicevox)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-voicevox)](https://github.com/shouni/go-voicevox/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/go-voicevox.svg)](https://pkg.go.dev/github.com/shouni/go-voicevox)

English | [日本語](README.ja.md)

## 🚀 About - takes ScriptLines, returns one combined WAV; saving is the caller's job

Go VOICEVOX is a Go library that generates speech from a structured script through the API of the **VOICEVOX engine**.

Its only responsibility is to **take `[]ScriptLine` and return the combined WAV bytes**. It neither writes files nor
uploads to cloud storage, and depends on neither. Where the result is saved is the caller's decision.

---

## ✨ Features

* **One function to wire it up** — `voicevox.New(...)` sets up the API client, the speaker data, the reading
  converter and the internal engine together. An empty `apiURL` falls back to `localhost:50021`.
* **No built-in speaker list** — who to use is application policy, so pass a saved `/speakers` response to
  `speaker.NewRegistry(raw)` (with `nil`, every speaker the engine provides is accepted).
  **Style IDs always come from the live engine** — they change between engine builds, so using saved IDs turns a
  missed update into "speaking in another character's voice".
* **Exposed vocabulary** — `Registry`'s `SpeakerNames()` / `StyleNames()` / `StylesFor(name)` /
  `DefaultStyleFor(name)`, for building AI response schemas and the like. **`StylesFor`, which looks up per
  speaker, is recommended.** `StyleNames()` is a union, so it lets the AI pick combinations that do not exist
  (those fall back to the default style, and the instruction is silently ignored).
* **Reading conversion is mandatory** — to avoid kanji that VOICEVOX tends to misread, text is always converted
  to a katakana reading before synthesis. The caller cannot turn it off.
* **Number readings** — `WithNumberReading()` converts Arabic numerals to their Japanese readings.
  **It is not enabled by default.** The morphological analyser's dictionary has no readings for Arabic numerals,
  so by default "2026年8月" becomes "2026ネン8ツキ" and VOICEVOX reads the digits literally
  (8日→ハチニチ, 1人→イチニン, 20歳→ニジュッサイ). With it, both the number readings and the sound changes of
  counters (一回→イッカイ, 三本→サンボン) come out right. **The converted text still shows the digits, so with
  it off a misreading goes unnoticed until synthesis.** Enable it for scripts that contain dates, head counts or
  ages.
* **Reading overrides** — `WithReadingOverrides(map[string]string{"8日": "ヨウカ"})` adds readings per spelling.
  **It is for proper nouns and readings the rules do not produce** — numbers and counters are read by rule with
  `WithNumberReading`, so reach for that first.
  Which words get which reading is application vocabulary, so, as with the speaker list, the library holds none.
* **Readings can be checked without an engine** — `voicevox.NewReadingPreview(opts...)` builds only the reading
  conversion from the same `Option`s as `New`, and `Read(text)` returns the reading for each of the same breaks
  as synthesis. Building a separate `phonetic.Converter` would leave two copies of the settings, and since it
  skips the split, long lines would differ from synthesis. Pass one `[]voicevox.Option` to both `New` and it.
* **Long text is split automatically** — segments are force-split at a 200-character limit before synthesis.
* **Control over parallel synthesis** — concurrency, rate and per-segment timeout are tuned with
  `WithMaxParallelSegments` / `WithSegmentRateLimit` / `WithSegmentTimeout`
  (defaults: 5 in parallel / 100ms interval / 180 seconds. If the defaults are fine, do not pass them).
  The submission interval is not a throughput knob; it smooths out the burst of connections at startup.
  **Output order matches input order.**
* **Errors are aggregated** — instead of stopping at the first failure, every segment's failure is returned
  together as one error. **If even one fails, `ErrSynthesisBatch` is returned and no audio with gaps is
  returned.**
* **Engine is immutable after construction** — one `Engine` can be `Run` from several goroutines at once.

---

## 🚦 Usage

```go
engine, err := voicevox.New(ctx, httpClient, apiURL, registry)
wavBytes, err := engine.Run(ctx, []voicevox.ScriptLine{
    {Speaker: "四国めたん", Style: "ノーマル", Text: "こんにちは。"},
})
os.WriteFile("out.wav", wavBytes, 0o644) // saving is the caller's responsibility
```

Normally these two calls are all there is. Preparing the speaker list (`speaker.NewRegistry`) and saving belong to the caller.

---

## 📦 Package Structure

The only entry point is `package voicevox`. Normally you use nothing but `voicevox.New(...)` and
`Engine.Run(ctx, lines)`.

```text
go-voicevox/
├── cmd/           # demo/sample CLI, not the library itself
│   └── voicevox-demo/
├── voicevox/      # public API; New wires the dependencies and returns an Engine
├── speaker/       # /speakers response structure, Registry, style ID resolution (Registry.LoadStyles)
└── internal/      # not for outside use
    ├── api/       #   VOICEVOX API calls (/audio_query, /synthesis, /speakers)
    └── engine/    #   segmentation, reading conversion, parallel synthesis, WAV combining, failure aggregation
```

---

## 📜 License

* The characters used are decided by the caller through `speaker.Registry`. This library itself neither bundles
  nor designates any particular character. When publishing synthesized audio, follow the terms of use of
  VOICEVOX itself and of each voice library (a credit is required).
* This project is released under the [MIT License](https://opensource.org/licenses/MIT).
