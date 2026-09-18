package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

func adminToken(secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("negotiation-arena-admin"))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validToken(token, secret string) bool {
	return hmac.Equal([]byte(token), []byte(adminToken(secret)))
}
