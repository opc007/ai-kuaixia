package sign

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// GenerateSign 生成签名（通用）
func GenerateSign(params map[string]string, secret string) string {
	// 1. 按key排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. 拼接字符串
	var signStr strings.Builder
	for i, k := range keys {
		if i > 0 {
			signStr.WriteString("&")
		}
		signStr.WriteString(k)
		signStr.WriteString("=")
		signStr.WriteString(params[k])
	}
	signStr.WriteString("&key=")
	signStr.WriteString(secret)

	// 3. SHA256加密
	h := sha256.New()
	h.Write([]byte(signStr.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// MD5 MD5加密
func MD5(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
