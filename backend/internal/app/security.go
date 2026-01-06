package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
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

func (a *App) encryptRconPassword(plaintext string) (string, error) {
	nonce := make([]byte, a.RconAead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	cipherText := a.RconAead.Seal(nil, nonce, []byte(plaintext), nil)
	buf := make([]byte, 0, len(nonce)+len(cipherText))
	buf = append(buf, nonce...)
	buf = append(buf, cipherText...)
	return base64.StdEncoding.EncodeToString(buf), nil
}

func (a *App) decryptRconPassword(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := a.RconAead.NonceSize()
	if len(raw) <= nonceSize {
		return "", errors.New("invalid ciphertext")
	}
	nonce, cipherText := raw[:nonceSize], raw[nonceSize:]
	plain, err := a.RconAead.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
}

var defaultArgon = argonParams{
	memory:  64 * 1024,
	time:    3,
	threads: 2,
	keyLen:  32,
}

type pbkdf2Params struct {
	iter   int
	keyLen int
}

var defaultPbkdf2 = pbkdf2Params{
	iter:   120000,
	keyLen: 32,
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := pbkdf2.Key([]byte(password), salt, defaultPbkdf2.iter, defaultPbkdf2.keyLen, sha256.New)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("pbkdf2_sha256$i=%d$%s$%s", defaultPbkdf2.iter, b64Salt, b64Hash), nil
}

func verifyPassword(password, encoded string) (bool, error) {
	if strings.HasPrefix(encoded, "pbkdf2_sha256$") {
		return verifyPasswordPBKDF2SHA256(password, encoded)
	}
	if strings.HasPrefix(encoded, "argon2id$") {
		return verifyPasswordArgon2id(password, encoded)
	}
	return false, errors.New("invalid hash format")
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

	var p argonParams
	p.keyLen = 32

	memory, timeCost, threads, err := parseArgonParams(parts[2])
	if err != nil {
		return false, err
	}
	p.memory = memory
	p.time = timeCost
	p.threads = threads

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	p.keyLen = uint32(len(hash))

	other := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	if subtleConstantTimeCompare(hash, other) {
		return true, nil
	}
	return false, nil
}

func parseArgonParams(s string) (memory uint32, timeCost uint32, threads uint8, err error) {
	memory = 0
	timeCost = 0
	threads = 0
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

