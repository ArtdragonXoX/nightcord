package utils

import (
	"math/rand/v2"
	"sync"
)

type LockedRandom struct {
	mu sync.Mutex
}

func (lr *LockedRandom) String(n int) string {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	return RandomString(n)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// RandomString 生成长度为 n 的随机字符串（包含字母和数字）
func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}
