package utils

import (
	"encoding/json"
	"unsafe"
)

func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return "", err
	}
	return string(val), nil
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
