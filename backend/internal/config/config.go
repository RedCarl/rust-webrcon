package config

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type AppConfig struct {
	BindAddr          string `json:"bind_addr"`
	JWTSecret         string `json:"jwt_secret"`
	RconEncryptionKey string `json:"rcon_encryption_key"`
	DataDir           string `json:"data_dir"`
	DockerBin         string `json:"docker_bin"`
}

func Load(path string) (AppConfig, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return AppConfig{}, errors.New("配置文件路径为空")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return initConfigFile(p)
		}
		return AppConfig{}, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return initConfigFile(p)
	}
	var cfg AppConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return AppConfig{}, err
	}
	cfg.Normalize()
	return cfg, nil
}

func initConfigFile(path string) (AppConfig, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return AppConfig{}, err
		}
	}

	jwtSecret := make([]byte, 32)
	if _, err := rand.Read(jwtSecret); err != nil {
		return AppConfig{}, err
	}

	rconKey := make([]byte, 32)
	if _, err := rand.Read(rconKey); err != nil {
		return AppConfig{}, err
	}

	cfg := AppConfig{
		BindAddr:          "0.0.0.0:8084",
		JWTSecret:         hex.EncodeToString(jwtSecret),
		RconEncryptionKey: base64.StdEncoding.EncodeToString(rconKey),
		DataDir:           ".",
		DockerBin:         "docker",
	}
	cfg.Normalize()

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return AppConfig{}, err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return AppConfig{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return AppConfig{}, err
	}
	return cfg, nil
}

func (c *AppConfig) Normalize() {
	c.BindAddr = strings.TrimSpace(c.BindAddr)
	if c.BindAddr == "" {
		c.BindAddr = "0.0.0.0:8084"
	}
	c.JWTSecret = strings.TrimSpace(c.JWTSecret)
	c.RconEncryptionKey = strings.TrimSpace(c.RconEncryptionKey)
	c.DataDir = strings.TrimSpace(c.DataDir)
	if c.DataDir == "" {
		c.DataDir = "."
	}
	c.DockerBin = strings.TrimSpace(c.DockerBin)
	if c.DockerBin == "" {
		c.DockerBin = "docker"
	}
}

func (c *AppConfig) Validate() error {
	if c.JWTSecret == "" {
		return errors.New("jwt_secret 不能为空")
	}
	if c.RconEncryptionKey == "" {
		return errors.New("rcon_encryption_key 不能为空")
	}
	if c.DataDir == "" {
		return errors.New("data_dir 不能为空")
	}
	return nil
}

func Parse32ByteKey(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if len(s) == 64 && isHexString(s) {
		b, err := hex.DecodeString(s)
		if err != nil || len(b) != 32 {
			return nil, errors.New("rcon_encryption_key 需要是 32 字节（base64/hex）")
		}
		return b, nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, errors.New("rcon_encryption_key 需要是 32 字节（base64/hex）")
	}
	return b, nil
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

