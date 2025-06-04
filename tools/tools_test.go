package tools

import (
	"errors"
	"os"
	"testing"
)

func TestCompressTargz(t *testing.T) {
	input := "./tools_test.go"
	output := "./tools_test.tar.gz"

	if err := CompressTargz(input, output); err != nil {
		t.Fatalf("error: %v", err)
	}

	if _, err := os.Stat(output); errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file %s not exists", output)
	}
}

func TestGetTimestamp(t *testing.T) {
	t.Logf(GetCurrentTimestampToSecond())
}
