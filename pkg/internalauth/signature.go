package internalauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"platform-service/pkg/platformconst"
)

func BuildMessage(service, method, path, timestamp string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	return BuildMessageWithBodyHash(service, method, path, timestamp, bodyHash[:])
}

func BuildMessageWithBodyHash(service, method, path, timestamp string, bodyHash []byte) string {
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", service, method, path, timestamp, hex.EncodeToString(bodyHash))
}

func Sign(secret, service, method, path, timestamp string, body []byte) string {
	message := BuildMessage(service, method, path, timestamp, body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret, signature, service, method, path, timestamp string, body []byte) bool {
	expected := Sign(secret, service, method, path, timestamp, body)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func VerifyBodyHash(secret, signature, service, method, path, timestamp string, bodyHash []byte) bool {
	message := BuildMessageWithBodyHash(service, method, path, timestamp, bodyHash)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

func BuildHeaders(secret, service, method, path string, body []byte) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	return map[string]string{
		platformconst.HeaderInternalService:   service,
		platformconst.HeaderInternalTimestamp: timestamp,
		platformconst.HeaderInternalSignature: Sign(secret, service, method, path, timestamp, body),
	}
}
