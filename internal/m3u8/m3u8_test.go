package m3u8

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func readTestData(fileName string) *os.File {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}
	rootDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	f, err := os.Open(filepath.Join(rootDir, "testdata", fileName))
	if err != nil {
		panic(err)
	}
	return f
}

func TestGetURI(t *testing.T) {
	expected := "https://radiko.jp/v2/api/ts/chunklist/NejwTOkX.m3u8"

	input := bufio.NewReader(readTestData("uri.m3u8"))
	u, err := GetURI(input)
	if err != nil {
		t.Error(err)
	}
	if u != expected {
		t.Errorf("expected %s, but %s", expected, u)
	}
}

func TestGetChunklist(t *testing.T) {
	input := bufio.NewReader(readTestData("chunklist.m3u8"))
	chunklist, err := GetChunklist(input)
	if err != nil {
		t.Error(err)
	}
	if len(chunklist) == 0 {
		t.Error("chunklist is empty.")
	}
}
