package voicevox

import "time"

// ----------------------------------------------------------------------
// エンジン処理定数
// ----------------------------------------------------------------------

const (
	defaultVoicevoxAPIURL      = "http://localhost:50021"
	DefaultMaxParallelSegments = 8
	DefaultSegmentTimeout      = 300 * time.Second
	DefaultSegmentRateLimit    = 1000 * time.Millisecond
)
