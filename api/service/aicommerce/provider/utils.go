package provider

import (
	"crypto/md5"
	"fmt"
)

func md5Sum(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}
