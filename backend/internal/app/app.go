package app

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"

	"rust-panel-backend/internal/config"
	"rust-panel-backend/internal/store"
)

type App struct {
	Store                   *store.JSONStore
	JWTSecret               []byte
	LegacyRconAead          cipher.AEAD
	DockerBin               string
	DataDir                 string
	AuthorLabel             string
	PlayerDataMySQLHost     string
	PlayerDataMySQLPort     int
	PlayerDataMySQLUser     string
	PlayerDataMySQLPassword string
	PlayerDataMySQLDatabase string
}

func New(cfg config.AppConfig) (*App, error) {
	jwtSecret := []byte(cfg.JWTSecret)
	var legacyRconAead cipher.AEAD
	if keyText := cfg.RconEncryptionKey; keyText != "" {
		rconKey, err := config.Parse32ByteKey(keyText)
		if err != nil {
			return nil, err
		}
		block, err := aes.NewCipher(rconKey)
		if err != nil {
			return nil, err
		}
		legacyRconAead, err = cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
	}

	st, err := store.OpenJSONStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if err := ensureDefaultUser(st); err != nil {
		return nil, err
	}
	app := &App{
		Store:                   st,
		JWTSecret:               jwtSecret,
		LegacyRconAead:          legacyRconAead,
		DockerBin:               cfg.DockerBin,
		DataDir:                 cfg.DataDir,
		AuthorLabel:             "CarlJoy",
		PlayerDataMySQLHost:     cfg.PlayerDataMySQLHost,
		PlayerDataMySQLPort:     cfg.PlayerDataMySQLPort,
		PlayerDataMySQLUser:     cfg.PlayerDataMySQLUser,
		PlayerDataMySQLPassword: cfg.PlayerDataMySQLPassword,
		PlayerDataMySQLDatabase: cfg.PlayerDataMySQLDatabase,
	}
	if err := app.migrateLegacySecrets(); err != nil {
		return nil, err
	}
	return app, nil
}

func ensureDefaultUser(st *store.JSONStore) error {
	if u, ok := st.FindUserByUsername("system"); ok {
		ok, err := verifyPassword("123456", u.Password)
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
		return st.UpdateUserPassword("system", hash)
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

func (a *App) migrateLegacySecrets() error {
	if a.LegacyRconAead == nil {
		return nil
	}

	for _, srv := range a.Store.ListServersAny() {
		plaintext, ok := a.tryDecryptLegacyRconPassword(srv.RconPassword)
		if !ok {
			continue
		}
		if _, err := a.Store.UpdateServerAny(srv.ID, func(next *store.Server) error {
			next.RconPassword = plaintext
			return nil
		}); err != nil {
			return err
		}
	}

	settings := a.Store.GetGlobalSettings()
	if settings.WipeConfig == nil {
		return nil
	}
	plaintext, ok := a.tryDecryptLegacyRconPassword(settings.WipeConfig.S3SecretKey)
	if !ok {
		return nil
	}
	_, err := a.Store.UpdateGlobalSettings(func(next *store.GlobalSettings) error {
		if next.WipeConfig == nil {
			next.WipeConfig = &store.WipeConfig{}
		}
		next.WipeConfig.S3SecretKey = plaintext
		return nil
	})
	return err
}
