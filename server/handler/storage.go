//go:build linux
// +build linux

package handler

import (
	"io"
	"net/http"
	"nightcord-server/internal/service/storage"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// StorageHandler 存储处理器
type StorageHandler struct {
	storageEngine *storage.StorageEngine
}

// NewStorageHandler 创建新的存储处理器
func NewStorageHandler() *StorageHandler {
	return &StorageHandler{
		storageEngine: storage.GetStorageEngineInstance(),
	}
}

// UploadFile 上传文件
func (h *StorageHandler) UploadFile(c *gin.Context) {
	filename := c.PostForm("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "filename is required",
		})
		return
	}

	// 验证文件名
	if !isValidFilename(filename) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid filename",
		})
		return
	}

	// 从表单获取测试用例内容
	testcase := c.PostForm("testcase")
	if testcase == "" {
		// 尝试从文件上传获取内容
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "testcase or file is required",
			})
			return
		}

		// 读取上传的文件内容
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to open uploaded file",
			})
			return
		}
		defer src.Close()

		testcaseBytes, err := io.ReadAll(src)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to read uploaded file",
			})
			return
		}
		testcase = string(testcaseBytes)
	}

	// 写入文件
	err := h.storageEngine.WriteFile(filename, []byte(testcase))
	if err != nil {
		if strings.Contains(err.Error(), "only testcase files are allowed") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "only testcase files are allowed",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "file uploaded successfully",
		"filename": filename,
	})
}

// DownloadFile 下载文件
func (h *StorageHandler) DownloadFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "filename is required",
		})
		return
	}

	// 获取测试用例内容
	reader, err := h.storageEngine.ReadFile(filename)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "file not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		return
	}
	defer reader.Close()

	// 获取文件元数据
	metadata, err := h.storageEngine.GetFileMetadata(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get file metadata",
		})
		return
	}

	// 设置响应头
	c.Header("Content-Type", metadata.ContentType)
	c.Header("Content-Length", strconv.FormatInt(metadata.Size, 10))
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")

	// 流式传输文件内容
	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to stream file content",
		})
		return
	}
}

// GetTestcaseContent 获取测试用例内容（以JSON格式返回）
func (h *StorageHandler) GetTestcaseContent(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "filename is required",
		})
		return
	}

	// 获取测试用例内容
	reader, err := h.storageEngine.ReadFile(filename)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "file not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		return
	}
	defer reader.Close()

	// 读取测试用例内容
	testcase, err := io.ReadAll(reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to read testcase content",
		})
		return
	}

	// 获取文件元数据
	metadata, err := h.storageEngine.GetFileMetadata(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get file metadata",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"filename": filename,
		"testcase": string(testcase),
		"metadata": metadata,
	})
}

// GetFileMetadata 获取文件元数据
func (h *StorageHandler) GetFileMetadata(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "filename is required",
		})
		return
	}

	metadata, err := h.storageEngine.GetFileMetadata(filename)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "file not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, metadata)
}

// ListFiles 列出所有文件
func (h *StorageHandler) ListFiles(c *gin.Context) {
	files, err := h.storageEngine.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"count": len(files),
	})
}

// DeleteFile 删除文件
func (h *StorageHandler) DeleteFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "filename is required",
		})
		return
	}

	err := h.storageEngine.DeleteFile(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "file deleted successfully",
		"filename": filename,
	})
}

// UpdateFile 更新文件内容
func (h *StorageHandler) UpdateFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "filename is required",
		})
		return
	}

	// 验证文件名
	if !isValidFilename(filename) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid filename",
		})
		return
	}

	// 获取新的测试用例内容
	testcase := c.PostForm("testcase")
	if testcase == "" {
		// 尝试从文件上传获取内容
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "testcase or file is required",
			})
			return
		}

		// 读取上传的文件内容
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to open uploaded file",
			})
			return
		}
		defer src.Close()

		testcaseBytes, err := io.ReadAll(src)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to read uploaded file",
			})
			return
		}
		testcase = string(testcaseBytes)
	}

	// 检查文件是否存在
	_, err := h.storageEngine.GetFileMetadata(filename)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "file not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		return
	}

	// 更新测试用例内容
	err = h.storageEngine.WriteFile(filename, []byte(testcase))
	if err != nil {
		if strings.Contains(err.Error(), "only testcase files are allowed") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "only testcase files are allowed",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "file updated successfully",
		"filename": filename,
	})
}

// isValidFilename 验证文件名是否有效
func isValidFilename(filename string) bool {
	if filename == "" {
		return false
	}

	// 检查是否包含非法字符
	invalidChars := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(filename, char) {
			return false
		}
	}

	// 检查文件名长度
	if len(filename) > 255 {
		return false
	}

	// 检查是否为相对路径
	if filepath.IsAbs(filename) {
		return false
	}

	return true
}
