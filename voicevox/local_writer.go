package voicevox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shouni/go-voicevox/internal/contracts"
)

// localWriter は、ローカルファイルシステムへ書き込む Writer 実装です。
// ContentType/Inline/CacheControl はローカルファイル出力には意味を持たないため無視します。
type localWriter struct{}

// NewLocalWriter は、ローカルファイルシステムへ書き込む Writer を返します。
// 出力先ディレクトリが存在しない場合は自動的に作成します。
func NewLocalWriter() Writer {
	return localWriter{}
}

func (localWriter) Write(_ context.Context, path string, contentReader io.Reader, _ ...contracts.WriteOption) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("出力ディレクトリの作成に失敗しました (%s): %w", dir, err)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("出力ファイルの作成に失敗しました (%s): %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, contentReader); err != nil {
		return fmt.Errorf("出力ファイルへの書き込みに失敗しました (%s): %w", path, err)
	}

	return nil
}
