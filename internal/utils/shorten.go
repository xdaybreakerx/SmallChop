package utils

import (
	"encoding/base64"
	"fmt"
	"time"
)

// Creates a short URL based on creation timestamp
func GetShortCode() string {
	fmt.Println("Shortening URL")

	ts := time.Now().UnixNano()
	fmt.Println("Timestamp: ", ts)

	// We convert the timestamp to byte slice and then encode it to base64 string
	ts_bytes := []byte(fmt.Sprintf("%d", ts))
	key := base64.StdEncoding.EncodeToString(ts_bytes)
	fmt.Println("Key: ", key)
	
	// We remove the last two chars since they are usuall always equal signs (==)
	key = key[:len(key)-2]
	// We return the last chars after 16 chars, these are almost always different
	return key[16:]
}
