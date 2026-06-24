package utils

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

// Md5Encode 小写MD5
func Md5Encode(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	tempStr := h.Sum(nil)
	return hex.EncodeToString(tempStr)
}

// MD5Encode 大写MD5
func MD5Encode(data string) string {
	return strings.ToUpper(Md5Encode(data))
}

// MakePassword 加密
func MakePassword(plainpwd, salt string) string {
	return MD5Encode(plainpwd + salt)
}

// ValidatePassword 解密
func ValidatePassword(plainpwd, salt string, password string) bool {
	return MD5Encode(plainpwd+salt) == password
}
