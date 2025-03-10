package utils

import (
	"encoding/json"
	"os"
	"unsafe"

	"math/rand/v2"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// RandomString 生成长度为 n 的随机字符串（包含字母和数字）
func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}

func IsFileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // 文件不存在，返回false和不为nil的error
		}
		return false, err // 其他错误，返回false和错误
	}
	return true, nil // 文件存在，返回true和nil的error
}

func EnsureDir(dir string) error {
	// 检查文件夹是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// 文件夹不存在，尝试创建文件夹
		err := os.MkdirAll(dir, 0755) // 0755是权限设置，表示所有者有读写执行权限，其他用户有读和执行权限
		if err != nil {
			return err // 如果创建失败，返回错误
		}
	}
	return nil // 文件夹存在或创建成功
}

func IsLittleEndian() bool {
	var value int32 = 1 // 占4byte 转换成16进制 0x00 00 00 01
	pointer := unsafe.Pointer(&value)
	pb := (*byte)(pointer)
	return *pb == 1
}

func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
