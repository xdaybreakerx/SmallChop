package utils

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Creates a short URL based on creation timestamp
// this has since been replaced with the encode/ decode functions
func GetShortCode() string {
	fmt.Println("Shortening URL")

	ts := time.Now().UnixNano()
	fmt.Println("Timestamp: ", ts)

	// We convert the timestamp to byte slice and then encode it to base64 string
	ts_bytes := []byte(fmt.Sprintf("%d", ts))
	key := base64.StdEncoding.EncodeToString(ts_bytes)
	fmt.Println("Key: ", key)

	// We remove the last two chars since they are always equal signs (==)
	key = key[:len(key)-2]
	// We return the last chars after 16 chars, these are almost always different
	return key[16:]
}

// alphabet is base 52 as this will not create any valid english words
const alphabet = "bcdfghjklmnpqrstvwxyzBCDFGHJKLMNPQRSTVWXYZ0123456789"
const base = int64(len(alphabet))

// base 52 encode function
func Encode(num int64) string {
	if num == 0 {
		return string(alphabet[0])
	}
	var encoded strings.Builder
	for num > 0 {
		remainder := num % base
		encoded.WriteByte(alphabet[remainder])
		num = num / base
	}
	// Reverse the encoded string
	result := []rune(encoded.String())
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

// base 52 decode function
func Decode(encoded string) int64 {
	var num int64
	for _, char := range encoded {
		index := strings.IndexRune(alphabet, char)
		if index == -1 {
			return -1 // Invalid character
		}
		num = num*base + int64(index)
	}
	return num
}

// basic URL sanitisation
func SanitizeURL(rawURL string) (string, error) {
	if len(rawURL) > 2048 {
		return "", errors.New("URL is too long")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("invalid URL format")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", errors.New("URL must start with http or https")
	}
	decodedPath, err := url.PathUnescape(parsedURL.Path)
	if err != nil {
		return "", errors.New("error decoding URL path")
	}
	decodedQuery, err := url.QueryUnescape(parsedURL.RawQuery)
	if err != nil {
		return "", errors.New("error decoding URL query")
	}
	combined := decodedPath + decodedQuery
	lowerCombined := strings.ToLower(combined)
	if strings.Contains(lowerCombined, "javascript:") || strings.Contains(lowerCombined, "<script>") {
		return "", errors.New("URL contains potentially malicious content")
	}
	sanitizedURL := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.RequestURI()
	return sanitizedURL, nil
}
