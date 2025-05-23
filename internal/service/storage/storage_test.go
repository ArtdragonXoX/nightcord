//go:build linux
// +build linux

package storage

import (
	"path/filepath"
	"testing"
)

func TestStorageEngine(t *testing.T) {
	// 创建临时目录用于测试
	tempDir := t.TempDir()
	storeDir := filepath.Join(tempDir, "files")
	dbPath := filepath.Join(tempDir, "test.db")

	// 创建存储引擎实例
	se, err := NewStorageEngine(storeDir, dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage engine: %v", err)
	}
	defer se.Close()

	// 测试写入文本文件
	testcase := "Hello, World!\nThis is a test file."
	testFilename := "test.txt"

	err = se.WriteFile(testFilename, []byte(testcase))
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// 测试读取文件
	reader, err := se.ReadFile(testFilename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	defer reader.Close()

	// 验证测试用例内容
	testcaseData := make([]byte, len(testcase))
	n, err := reader.Read(testcaseData)
	if err != nil {
		t.Fatalf("Failed to read testcase content: %v", err)
	}

	if string(testcaseData[:n]) != testcase {
		t.Errorf("Testcase content mismatch. Expected: %s, Got: %s", testcase, string(testcaseData[:n]))
	}

	// 测试获取文件元数据
	metadata, err := se.GetFileMetadata(testFilename)
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}

	if metadata.Filename != testFilename {
		t.Errorf("Filename mismatch. Expected: %s, Got: %s", testFilename, metadata.Filename)
	}

	if metadata.Size != int64(len(testcase)) {
		t.Errorf("File size mismatch. Expected: %d, Got: %d", len(testcase), metadata.Size)
	}

	// 测试列出文件
	files, err := se.ListFiles()
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	// 测试删除文件
	err = se.DeleteFile(testFilename)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// 验证文件已删除
	_, err = se.ReadFile(testFilename)
	if err == nil {
		t.Error("Expected error when reading deleted file")
	}
}

func TestIsTestcaseFile(t *testing.T) {
	tempDir := t.TempDir()
	storeDir := filepath.Join(tempDir, "files")
	dbPath := filepath.Join(tempDir, "test.db")

	se, err := NewStorageEngine(storeDir, dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage engine: %v", err)
	}
	defer se.Close()

	// 测试有效的测试用例文件
	testcaseContent := []byte("This is a valid testcase file\nwith multiple lines.")
	if !se.isTestcaseFile(testcaseContent) {
		t.Error("Valid testcase content was rejected")
	}

	// 测试二进制文件（模拟）
	binaryContent := make([]byte, 100)
	for i := range binaryContent {
		binaryContent[i] = byte(i % 256)
	}
	if se.isTestcaseFile(binaryContent) {
		t.Error("Binary content was accepted as testcase")
	}

	// 测试空文件
	emptyContent := []byte("")
	if !se.isTestcaseFile(emptyContent) {
		t.Error("Empty content was rejected")
	}
}

func TestWriteNonTestcaseFile(t *testing.T) {
	tempDir := t.TempDir()
	storeDir := filepath.Join(tempDir, "files")
	dbPath := filepath.Join(tempDir, "test.db")

	se, err := NewStorageEngine(storeDir, dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage engine: %v", err)
	}
	defer se.Close()

	// 尝试写入二进制文件
	binaryContent := make([]byte, 100)
	for i := range binaryContent {
		binaryContent[i] = byte(i % 256)
	}

	err = se.WriteFile("binary.bin", binaryContent)
	if err == nil {
		t.Error("Expected error when writing binary file")
	}

	if err.Error() != "only testcase files are allowed" {
		t.Errorf("Expected 'only testcase files are allowed' error, got: %v", err)
	}
}
