package utils

import "os"

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
