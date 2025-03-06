package utils_test

import (
	"math/rand/v2"
	"nightcord-server/utils"
	"testing"
)

func TestRandomString(t *testing.T) {
	for range 10 {
		l := rand.IntN(256)
		str := utils.RandomString(l)
		if len(str) != l {
			t.Errorf("The length of the random string is %v instead of %v.", len(str), l)
		} else {
			t.Logf("Random string successfully generated: %v", str)
		}
	}
}
