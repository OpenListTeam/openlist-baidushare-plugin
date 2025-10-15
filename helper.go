package main

import (
	"crypto/rc4"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

const (
	deviceID            = "BB91C9B818963851F99A99261A70E37E|VUFQKX5JL"
	appSignatureCertMD5 = "ae5821440fab5e1a61a025f014bd8972"
	appVersion          = "11.10.4"
	hmacDynamicKey      = "B8ec24caf34ef7227c66767d29ffd3fb"
)

// getTrueSK
func getTrueSK(encryptedSK, uid string) (string, error) {
	decodedSKBytes, err := base64.StdEncoding.DecodeString(encryptedSK)
	if err != nil {
		return "", fmt.Errorf("failed to base64 decode SK: %w", err)
	}
	keyBytes := []byte(uid)
	cipher, err := rc4.NewCipher(keyBytes) // 标准库
	if err != nil {
		return "", fmt.Errorf("failed to create ARC4 cipher: %w", err)
	}
	decryptedBytes := make([]byte, len(decodedSKBytes))
	cipher.XORKeyStream(decryptedBytes, decodedSKBytes)
	return string(decryptedBytes), nil
}

// hmacSha1PureGo 是一个纯 Go 实现的 HMAC-SHA1, 用于在 TinyGo 中替代不兼容的 crypto/hmac
func hmacSha1PureGo(key, data []byte) []byte {
	const blockSize = 64
	if len(key) > blockSize {
		h := sha1.New()
		h.Write(key)
		key = h.Sum(nil)
	}
	if len(key) < blockSize {
		key = append(key, make([]byte, blockSize-len(key))...)
	}

	oKeyPad := make([]byte, blockSize)
	iKeyPad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		oKeyPad[i] = key[i] ^ 0x5c
		iKeyPad[i] = key[i] ^ 0x36
	}

	h := sha1.New()
	h.Write(iKeyPad)
	h.Write(data)
	innerHash := h.Sum(nil)

	h.Reset()
	h.Write(oKeyPad)
	h.Write(innerHash)
	return h.Sum(nil)
}

// generateSharedownloadSign
func generateSharedownloadSign(postBodyString string, timestamp int64) string {
	baseString := fmt.Sprintf("%s_%s_%d", postBodyString, deviceID, timestamp)
	hmacHash := hmacSha1PureGo([]byte(hmacDynamicKey), []byte(baseString))
	return hex.EncodeToString(hmacHash)
}

// DecodeSceKey 对scekey进行解码处理，替换特定字符
func DecodeSceKey(scekey string) string {
	// 依次替换字符：-→+，~→=，_→/
	result := strings.ReplaceAll(scekey, "-", "+")
	result = strings.ReplaceAll(result, "~", "=")
	result = strings.ReplaceAll(result, "_", "/")
	return result
}

// ReplaceSizeAndQuality 同时替换URL中的size和quality参数
// size格式固定为"c{限制宽}_u{限制高}"，quality为数值
func ReplaceSizeAndQuality(originalURL string, width, height, quality int) (string, error) {
	// 解析URL
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return "", errors.New("解析URL失败: " + err.Error())
	}

	// 获取查询参数
	query := parsedURL.Query()

	// 构造并设置size参数（固定格式）
	newSize := fmt.Sprintf("c%d_u%d", width, height)
	query.Set("size", newSize)

	// 设置quality参数
	query.Set("quality", fmt.Sprintf("%d", quality))

	// 更新查询参数
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

func DecryptMd5(encryptMd5 string) string {
	if _, err := hex.DecodeString(encryptMd5); err == nil {
		return encryptMd5
	}

	var out strings.Builder
	out.Grow(len(encryptMd5))
	for i, n := 0, int64(0); i < len(encryptMd5); i++ {
		if i == 9 {
			n = int64(unicode.ToLower(rune(encryptMd5[i])) - 'g')
		} else {
			n, _ = strconv.ParseInt(encryptMd5[i:i+1], 16, 64)
		}
		out.WriteString(strconv.FormatInt(n^int64(15&i), 16))
	}

	encryptMd5 = out.String()
	return encryptMd5[8:16] + encryptMd5[:8] + encryptMd5[24:32] + encryptMd5[16:24]
}

func EncryptMd5(originalMd5 string) string {
	reversed := originalMd5[8:16] + originalMd5[:8] + originalMd5[24:32] + originalMd5[16:24]

	var out strings.Builder
	out.Grow(len(reversed))
	for i, n := 0, int64(0); i < len(reversed); i++ {
		n, _ = strconv.ParseInt(reversed[i:i+1], 16, 64)
		n ^= int64(15 & i)
		if i == 9 {
			out.WriteRune(rune(n) + 'g')
		} else {
			out.WriteString(strconv.FormatInt(n, 16))
		}
	}
	return out.String()
}
