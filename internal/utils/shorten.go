package utils

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
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
	// Positive IDs have one spelling. The first alphabet character is zero.
	if encoded == "" || encoded[0] == alphabet[0] {
		return -1
	}
	var num int64
	for _, char := range encoded {
		index := strings.IndexRune(alphabet, char)
		if index == -1 || num > (math.MaxInt64-int64(index))/base {
			return -1 // Invalid character or overflow.
		}
		num = num*base + int64(index)
	}
	return num
}

var ErrInvalidURL = errors.New("invalid destination URL")

// SanitizeURL validates a destination without changing its URL semantics.
func SanitizeURL(rawURL string) (string, error) {
	if len(rawURL) > 2048 {
		return "", fmt.Errorf("%w: URL is too long", ErrInvalidURL)
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.Opaque != "" ||
		strings.ContainsAny(rawURL, "\r\n\t ") {
		return "", fmt.Errorf("%w: expected an absolute URL without credentials or whitespace", ErrInvalidURL)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("%w: URL must use http or https", ErrInvalidURL)
	}
	if port := parsedURL.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("%w: invalid port", ErrInvalidURL)
		}
	}
	if strings.HasSuffix(parsedURL.Host, ":") {
		return "", fmt.Errorf("%w: empty port", ErrInvalidURL)
	}
	if _, err := url.QueryUnescape(parsedURL.RawQuery); err != nil {
		return "", fmt.Errorf("%w: malformed query escaping", ErrInvalidURL)
	}
	return rawURL, nil
}
