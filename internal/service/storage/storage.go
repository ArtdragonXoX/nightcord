//go:build linux
// +build linux

package storage

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
)

// FileMetadata 文件元数据结构
type FileMetadata struct {
	ID          int64     `json:"id"`
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StorageEngine 存储引擎结构
type StorageEngine struct {
	db       *sql.DB
	storeDir string
}

// NewStorageEngine 创建新的存储引擎实例
func NewStorageEngine(storeDir, dbPath string) (*StorageEngine, error) {
	// 确保存储目录存在
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %v", err)
	}

	// 打开SQLite数据库
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	se := &StorageEngine{
		db:       db,
		storeDir: storeDir,
	}

	// 初始化数据库表
	if err := se.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	return se, nil
}

// initDB 初始化数据库表
func (se *StorageEngine) initDB() error {
	query := `
	CREATE TABLE IF NOT EXISTS file_metadata (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		size INTEGER NOT NULL,
		content_type TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := se.db.Exec(query)
	return err
}

// isTestcaseFile 检查文件是否为测试用例文件
func (se *StorageEngine) isTestcaseFile(content []byte) bool {
	// 对于测试用例文件，只检查是否为有效的UTF-8编码
	// 不关心控制字符，因为测试用例可能包含各种特殊字符
	if !utf8.Valid(content) {
		return false
	}

	return true
}

// WriteFile 写入文件（新建或修改）
func (se *StorageEngine) WriteFile(filename string, content []byte) error {
	// 检查是否为测试用例文件
	if !se.isTestcaseFile(content) {
		return fmt.Errorf("only testcase files are allowed")
	}

	// 构建文件路径
	filePath := filepath.Join(se.storeDir, filename)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	// 更新数据库元数据
	return se.updateMetadata(filename, filePath, int64(len(content)))
}

// updateMetadata 更新文件元数据
func (se *StorageEngine) updateMetadata(filename, path string, size int64) error {
	// 检查文件是否已存在
	var exists bool
	err := se.db.QueryRow("SELECT EXISTS(SELECT 1 FROM file_metadata WHERE path = ?)", path).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// 更新现有记录
		_, err = se.db.Exec(
			"UPDATE file_metadata SET size = ?, updated_at = CURRENT_TIMESTAMP WHERE path = ?",
			size, path,
		)
	} else {
		// 插入新记录
		_, err = se.db.Exec(
			"INSERT INTO file_metadata (filename, path, size, content_type) VALUES (?, ?, ?, ?)",
			filename, path, size, "text/plain",
		)
	}

	return err
}

// ReadFile 读取文件并返回Reader接口
func (se *StorageEngine) ReadFile(filename string) (io.ReadCloser, error) {
	filePath := filepath.Join(se.storeDir, filename)

	// 检查文件是否存在于数据库中
	var exists bool
	err := se.db.QueryRow("SELECT EXISTS(SELECT 1 FROM file_metadata WHERE path = ?)", filePath).Scan(&exists)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("file not found in storage")
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}

	return file, nil
}

// GetFileMetadata 获取文件元数据
func (se *StorageEngine) GetFileMetadata(filename string) (*FileMetadata, error) {
	filePath := filepath.Join(se.storeDir, filename)

	var metadata FileMetadata
	err := se.db.QueryRow(
		"SELECT id, filename, path, size, content_type, created_at, updated_at FROM file_metadata WHERE path = ?",
		filePath,
	).Scan(
		&metadata.ID,
		&metadata.Filename,
		&metadata.Path,
		&metadata.Size,
		&metadata.ContentType,
		&metadata.CreatedAt,
		&metadata.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not found")
		}
		return nil, err
	}

	return &metadata, nil
}

// ListFiles 列出所有文件
func (se *StorageEngine) ListFiles() ([]*FileMetadata, error) {
	rows, err := se.db.Query(
		"SELECT id, filename, path, size, content_type, created_at, updated_at FROM file_metadata ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*FileMetadata
	for rows.Next() {
		var metadata FileMetadata
		err := rows.Scan(
			&metadata.ID,
			&metadata.Filename,
			&metadata.Path,
			&metadata.Size,
			&metadata.ContentType,
			&metadata.CreatedAt,
			&metadata.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, &metadata)
	}

	return files, nil
}

// DeleteFile 删除文件
func (se *StorageEngine) DeleteFile(filename string) error {
	filePath := filepath.Join(se.storeDir, filename)

	// 从数据库中删除记录
	_, err := se.db.Exec("DELETE FROM file_metadata WHERE path = ?", filePath)
	if err != nil {
		return err
	}

	// 删除物理文件
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %v", err)
	}

	return nil
}

// Close 关闭存储引擎
func (se *StorageEngine) Close() error {
	return se.db.Close()
}

// GetContentType 根据文件扩展名获取内容类型
func (se *StorageEngine) GetContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".in":
		return "text/x-testcase-input"
	case ".out":
		return "text/x-testcase-output"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".java":
		return "text/x-java"
	case ".c":
		return "text/x-c"
	case ".cpp", ".cc", ".cxx":
		return "text/x-c++"
	default:
		return "text/plain"
	}
}
