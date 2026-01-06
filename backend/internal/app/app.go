package app

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"

	"rust-panel-backend/internal/config"
	"rust-panel-backend/internal/store"
)

type App struct {
	Store       *store.JSONStore
	JWTSecret   []byte
	RconAead    cipher.AEAD
	DockerBin   string
	AuthorLabel string
}

func New(cfg config.AppConfig) (*App, error) {
	jwtSecret := []byte(cfg.JWTSecret)
	rconKey, err := config.Parse32ByteKey(cfg.RconEncryptionKey)
	if err != nil {
		return nil, err
	}

	st, err := store.OpenJSONStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if err := ensureDefaultUser(st); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(rconKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &App{
		Store:       st,
		JWTSecret:   jwtSecret,
		RconAead:    aead,
		DockerBin:   cfg.DockerBin,
		AuthorLabel: "CarlJoy",
	}, nil
}

func ensureDefaultUser(st *store.JSONStore) error {
	if u, ok := st.FindUserByUsername("system"); ok {
		ok, err := verifyPassword("123456", u.PasswordHash)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		hash, err := hashPassword("123456")
		if err != nil {
			return err
		}
		return st.UpdateUserPasswordHash("system", hash)
	}
	hash, err := hashPassword("123456")
	if err != nil {
		return err
	}
	if _, err := st.CreateUser("system", hash); err != nil {
		if errors.Is(err, store.ErrUserExists) {
			return nil
		}
		return err
	}
	return nil
}

