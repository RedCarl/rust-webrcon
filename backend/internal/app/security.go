package app

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

func signJWT(secret []byte, userID uint64) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": now.Add(7 * 24 * time.Hour).Unix(),
		"iat": now.Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(secret)
}

func (a *App) storeRconPassword(plaintext string) (string, error) {
	return plaintext, nil
}

func (a *App) readRconPassword(encoded string) (string, error) {
	if plaintext, ok := a.tryDecryptLegacyRconPassword(encoded); ok {
		return plaintext, nil
	}
	return encoded, nil
}

func (a *App) tryDecryptLegacyRconPassword(encoded string) (string, bool) {
	if a.LegacyRconAead == nil {
		return "", false
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	nonceSize := a.LegacyRconAead.NonceSize()
	if len(raw) <= nonceSize {
		return "", false
	}
	nonce, storedText := raw[:nonceSize], raw[nonceSize:]
	plain, err := a.LegacyRconAead.Open(nil, nonce, storedText, nil)
	if err != nil {
		return "", false
	}
	return string(plain), true
}

func hashPassword(password string) (string, error) {
	return password, nil
}

func verifyPassword(password, encoded string) (bool, error) {
	if password == encoded {
		return true, nil
	}
	if strings.HasPrefix(encoded, "pbkdf2_sha256$") {
		return verifyPasswordPBKDF2SHA256(password, encoded)
	}
	if strings.HasPrefix(encoded, "argon2id$") {
		return verifyPasswordArgon2id(password, encoded)
	}
	return false, nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
}

type pbkdf2Params struct {
	iter   int
	keyLen int
}

var defaultPbkdf2 = pbkdf2Params{
	iter:   120000,
	keyLen: 32,
}

func verifyPasswordPBKDF2SHA256(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false, errors.New("invalid hash format")
	}
	if parts[0] != "pbkdf2_sha256" {
		return false, errors.New("invalid hash format")
	}
	if !strings.HasPrefix(parts[1], "i=") {
		return false, errors.New("invalid hash format")
	}
	iterAny, err := strconv.ParseUint(strings.TrimPrefix(parts[1], "i="), 10, 32)
	if err != nil {
		return false, errors.New("invalid hash format")
	}
	iter := int(iterAny)
	if iter <= 0 {
		return false, errors.New("invalid hash format")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, errors.New("invalid hash format")
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, errors.New("invalid hash format")
	}
	other := pbkdf2.Key([]byte(password), salt, iter, len(hash), sha256.New)
	return subtleConstantTimeCompare(hash, other), nil
}

func verifyPasswordArgon2id(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid hash format")
	}
	if parts[0] != "argon2id" {
		return false, errors.New("invalid hash format")
	}
	if parts[1] != "v=19" {
		return false, errors.New("invalid hash format")
	}

	memory, timeCost, threads, err := parseArgonParams(parts[2])
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	other := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(hash)))
	return subtleConstantTimeCompare(hash, other), nil
}

func parseArgonParams(s string) (memory uint32, timeCost uint32, threads uint8, err error) {
	items := strings.Split(s, ",")
	for _, it := range items {
		it = strings.TrimSpace(it)
		if strings.HasPrefix(it, "m=") {
			v, e := strconv.ParseUint(strings.TrimPrefix(it, "m="), 10, 32)
			if e != nil {
				return 0, 0, 0, e
			}
			memory = uint32(v)
		} else if strings.HasPrefix(it, "t=") {
			v, e := strconv.ParseUint(strings.TrimPrefix(it, "t="), 10, 32)
			if e != nil {
				return 0, 0, 0, e
			}
			timeCost = uint32(v)
		} else if strings.HasPrefix(it, "p=") {
			v, e := strconv.ParseUint(strings.TrimPrefix(it, "p="), 10, 8)
			if e != nil {
				return 0, 0, 0, e
			}
			threads = uint8(v)
		}
	}
	if memory == 0 || timeCost == 0 || threads == 0 {
		return 0, 0, 0, errors.New("invalid hash params")
	}
	return memory, timeCost, threads, nil
}

func subtleConstantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var out byte
	for i := 0; i < len(a); i++ {
		out |= a[i] ^ b[i]
	}
	return out == 0
}
