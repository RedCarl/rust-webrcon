package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

type appState struct {
	db          *sql.DB
	jwtSecret   []byte
	rconAead    cipher.AEAD
	authorLabel string
}

type apiErrorBody struct {
	Message string `json:"message"`
}

func main() {
	dsn := mustEnv("DATABASE_URL")
	jwtSecret := []byte(mustEnv("JWT_SECRET"))
	rconKey := must32ByteKey(mustEnv("RCON_ENCRYPTION_KEY"))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败：%v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("连接数据库失败：%v", err)
	}

	if err := bootstrapSchema(db); err != nil {
		log.Fatalf("初始化数据库结构失败：%v", err)
	}

	block, err := aes.NewCipher(rconKey)
	if err != nil {
		log.Fatalf("初始化加密器失败：%v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		log.Fatalf("初始化加密器失败：%v", err)
	}

	state := &appState{
		db:          db,
		jwtSecret:   jwtSecret,
		rconAead:    aead,
		authorLabel: "CarlJoy",
	}

	addr := getenvDefault("BIND_ADDR", "0.0.0.0:3000")
	log.Printf("[%s] 后端启动：%s", state.authorLabel, addr)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	router.Static("/assets", "./dist/assets")
	router.StaticFile("/", "./dist/index.html")
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, apiErrorBody{Message: "未找到"})
			return
		}
		c.File("./dist/index.html")
	})

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", state.ginRegister)
			auth.POST("/login", state.ginLogin)
		}

		protected := api.Group("/")
		protected.Use(state.ginAuthRequired())
		{
			protected.GET("/me", state.ginMe)
			protected.GET("/servers", state.ginListServers)
			protected.POST("/servers", state.ginCreateServer)
			protected.PUT("/servers/:id", state.ginUpdateServer)
			protected.DELETE("/servers/:id", state.ginDeleteServer)
			protected.POST("/servers/:id/actions/:action", state.ginServerAction)
		}
	}

	if err := router.Run(addr); err != nil {
		log.Fatalf("服务异常退出：%v", err)
	}
}

func mustEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("缺少环境变量：%s", key)
	}
	return v
}

func getenvDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func must32ByteKey(raw string) []byte {
	s := strings.TrimSpace(raw)
	if len(s) == 64 && isHexString(s) {
		b, err := hex.DecodeString(s)
		if err != nil || len(b) != 32 {
			log.Fatalf("RCON_ENCRYPTION_KEY 需要是 32 字节（base64/hex）")
		}
		return b
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) != 32 {
		log.Fatalf("RCON_ENCRYPTION_KEY 需要是 32 字节（base64/hex）")
	}
	return b
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.Status(204)
			c.Abort()
			return
		}
		c.Next()
	}
}

func bootstrapSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(64) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS servers (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(128) NOT NULL,
  host VARCHAR(255) NOT NULL,
  port INT UNSIGNED NOT NULL,
  rcon_password_ciphertext TEXT NOT NULL,
  container_name VARCHAR(128) NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_user_id (user_id),
  CONSTRAINT fk_servers_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`)
	return err
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerResponse struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

func (s *appState) ginRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" {
		c.JSON(400, apiErrorBody{Message: "用户名不能为空"})
		return
	}
	if len(password) < 6 {
		c.JSON(400, apiErrorBody{Message: "密码至少 6 位"})
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "生成密码哈希失败"})
		return
	}

	res, err := s.db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, hash)
	if err != nil {
		if isMySQLDuplicate(err) {
			c.JSON(409, apiErrorBody{Message: "用户名已存在"})
			return
		}
		c.JSON(500, apiErrorBody{Message: "创建用户失败"})
		return
	}

	id, _ := res.LastInsertId()
	c.JSON(200, registerResponse{ID: uint64(id), Username: username})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (s *appState) ginLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
		return
	}

	var id uint64
	var passwordHash string
	err := s.db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &passwordHash)
	if err != nil {
		c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
		return
	}

	ok, err := verifyPassword(password, passwordHash)
	if err != nil || !ok {
		c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
		return
	}

	token, err := signJWT(s.jwtSecret, id)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "生成 token 失败"})
		return
	}

	c.JSON(200, loginResponse{Token: token})
}

type meResponse struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

func (s *appState) ginMe(c *gin.Context) {
	userID := mustUserID(c)
	var id uint64
	var username string
	err := s.db.QueryRow("SELECT id, username FROM users WHERE id = ?", userID).Scan(&id, &username)
	if err != nil {
		c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
		return
	}
	c.JSON(200, meResponse{ID: id, Username: username})
}

type serverDTO struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	Host          string  `json:"host"`
	Port          uint16  `json:"port"`
	RconPassword  string  `json:"rcon_password"`
	ContainerName *string `json:"container_name"`
}

func (s *appState) ginListServers(c *gin.Context) {
	userID := mustUserID(c)
	rows, err := s.db.Query("SELECT id, name, host, port, rcon_password_ciphertext, container_name FROM servers WHERE user_id = ? ORDER BY id DESC", userID)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "查询服务器失败"})
		return
	}
	defer rows.Close()

	var out []serverDTO
	for rows.Next() {
		var (
			id           uint64
			name         string
			host         string
			port         uint32
			cipherText   string
			containerNil sql.NullString
		)
		if err := rows.Scan(&id, &name, &host, &port, &cipherText, &containerNil); err != nil {
			c.JSON(500, apiErrorBody{Message: "查询服务器失败"})
			return
		}
		pw, err := s.decryptRconPassword(cipherText)
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "查询服务器失败"})
			return
		}
		var containerName *string
		if containerNil.Valid {
			v := strings.TrimSpace(containerNil.String)
			if v != "" {
				containerName = &v
			}
		}
		if port > 65535 {
			continue
		}
		out = append(out, serverDTO{
			ID:            id,
			Name:          name,
			Host:          host,
			Port:          uint16(port),
			RconPassword:  pw,
			ContainerName: containerName,
		})
	}
	c.JSON(200, out)
}

type createServerRequest struct {
	Name          string  `json:"name"`
	Host          string  `json:"host"`
	Port          uint16  `json:"port"`
	RconPassword  string  `json:"rcon_password"`
	ContainerName *string `json:"container_name"`
}

func (s *appState) ginCreateServer(c *gin.Context) {
	userID := mustUserID(c)
	var req createServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	host := strings.TrimSpace(req.Host)
	pw := strings.TrimSpace(req.RconPassword)
	if name == "" {
		c.JSON(400, apiErrorBody{Message: "服务器名称不能为空"})
		return
	}
	if host == "" {
		c.JSON(400, apiErrorBody{Message: "服务器地址不能为空"})
		return
	}
	if pw == "" {
		c.JSON(400, apiErrorBody{Message: "RCON 密码不能为空"})
		return
	}
	containerName := normalizeOptionalString(req.ContainerName)

	cipherText, err := s.encryptRconPassword(pw)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}

	res, err := s.db.Exec("INSERT INTO servers (user_id, name, host, port, rcon_password_ciphertext, container_name) VALUES (?, ?, ?, ?, ?, ?)",
		userID, name, host, uint32(req.Port), cipherText, containerName,
	)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}
	id, _ := res.LastInsertId()

	c.JSON(200, serverDTO{
		ID:            uint64(id),
		Name:          name,
		Host:          host,
		Port:          req.Port,
		RconPassword:  pw,
		ContainerName: containerName,
	})
}

type updateServerRequest struct {
	Name          *string `json:"name"`
	Host          *string `json:"host"`
	Port          *uint16 `json:"port"`
	RconPassword  *string `json:"rcon_password"`
	ContainerName *string `json:"container_name"`
}

func (s *appState) ginUpdateServer(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	var current struct {
		name       string
		host       string
		port       uint32
		cipherText string
		container  sql.NullString
	}
	err = s.db.QueryRow("SELECT name, host, port, rcon_password_ciphertext, container_name FROM servers WHERE id = ? AND user_id = ?", id, userID).
		Scan(&current.name, &current.host, &current.port, &current.cipherText, &current.container)
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	var req updateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}

	name := current.name
	host := current.host
	port := current.port
	cipherText := current.cipherText
	containerName := normalizeOptionalStringPtrFromNull(current.container)

	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		if v == "" {
			c.JSON(400, apiErrorBody{Message: "服务器名称不能为空"})
			return
		}
		name = v
	}
	if req.Host != nil {
		v := strings.TrimSpace(*req.Host)
		if v == "" {
			c.JSON(400, apiErrorBody{Message: "服务器地址不能为空"})
			return
		}
		host = v
	}
	if req.Port != nil {
		port = uint32(*req.Port)
	}
	if req.RconPassword != nil {
		v := strings.TrimSpace(*req.RconPassword)
		if v == "" {
			c.JSON(400, apiErrorBody{Message: "RCON 密码不能为空"})
			return
		}
		var err error
		cipherText, err = s.encryptRconPassword(v)
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
			return
		}
	}
	if req.ContainerName != nil {
		containerName = normalizeOptionalString(req.ContainerName)
	}

	_, err = s.db.Exec("UPDATE servers SET name = ?, host = ?, port = ?, rcon_password_ciphertext = ?, container_name = ? WHERE id = ? AND user_id = ?",
		name, host, port, cipherText, containerName, id, userID,
	)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}

	pw, err := s.decryptRconPassword(cipherText)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}
	if port > 65535 {
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}

	c.JSON(200, serverDTO{
		ID:            id,
		Name:          name,
		Host:          host,
		Port:          uint16(port),
		RconPassword:  pw,
		ContainerName: containerName,
	})
}

func (s *appState) ginDeleteServer(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	res, err := s.db.Exec("DELETE FROM servers WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "删除服务器失败"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	c.Status(204)
}

func normalizeOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func normalizeOptionalStringPtrFromNull(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := strings.TrimSpace(v.String)
	if s == "" {
		return nil
	}
	return &s
}

func (s *appState) ginAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := s.authedUserID(c.Request.Context(), c.GetHeader("Authorization"))
		if err != nil {
			c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
			c.Abort()
			return
		}
		c.Set("userID", userID)
		c.Next()
	}
}

func mustUserID(c *gin.Context) uint64 {
	v, ok := c.Get("userID")
	if !ok {
		return 0
	}
	id, _ := v.(uint64)
	return id
}

func parseUint64Param(c *gin.Context, key string) (uint64, error) {
	raw := strings.TrimSpace(c.Param(key))
	if raw == "" {
		return 0, errors.New("missing")
	}
	return strconv.ParseUint(raw, 10, 64)
}

type serverActionResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

var dockerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func (s *appState) ginServerAction(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	action := strings.TrimSpace(c.Param("action"))
	dockerArg, ok := mapServerAction(action)
	if !ok {
		c.JSON(400, apiErrorBody{Message: "不支持的操作"})
		return
	}

	var container sql.NullString
	err = s.db.QueryRow("SELECT container_name FROM servers WHERE id = ? AND user_id = ?", id, userID).Scan(&container)
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	if !container.Valid || strings.TrimSpace(container.String) == "" {
		c.JSON(400, apiErrorBody{Message: "该服务器未配置容器名称"})
		return
	}

	containerName := strings.TrimSpace(container.String)
	if !dockerNameRe.MatchString(containerName) {
		c.JSON(400, apiErrorBody{Message: "容器名称格式不合法"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()

	stdout, stderr, exitCode, err := dockerExecRustServer(ctx, containerName, dockerArg)
	if err != nil && exitCode == 0 {
		c.JSON(500, apiErrorBody{Message: "执行失败"})
		return
	}
	c.JSON(200, serverActionResponse{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	})
}

func mapServerAction(action string) (string, bool) {
	switch action {
	case "wipe":
		return "w", true
	case "update":
		return "u", true
	default:
		return "", false
	}
}

func dockerExecRustServer(ctx context.Context, containerName string, rustServerArg string) (stdout string, stderr string, exitCode int, err error) {
	dockerBin := getenvDefault("DOCKER_BIN", "docker")
	if _, lookErr := exec.LookPath(dockerBin); lookErr != nil {
		return "", "", 127, lookErr
	}

	cmd := exec.CommandContext(ctx, dockerBin, "exec", "-u", "linuxgsm", containerName, "./rustserver", rustServerArg)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = limitOutput(outBuf.String(), 64*1024)
	stderr = limitOutput(errBuf.String(), 64*1024)
	if runErr == nil {
		return stdout, stderr, 0, nil
	}

	if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return stdout, stderr, 124, runErr
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return stdout, stderr, ee.ExitCode(), runErr
	}
	return stdout, stderr, 1, runErr
}

func limitOutput(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func (s *appState) authedUserID(ctx context.Context, authHeader string) (uint64, error) {
	token := strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return 0, errors.New("missing token")
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("invalid alg")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return 0, errors.New("invalid token")
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid claims")
	}
	sub, ok := claims["sub"]
	if !ok {
		return 0, errors.New("missing sub")
	}

	var userID uint64
	switch v := sub.(type) {
	case float64:
		if v < 0 {
			return 0, errors.New("invalid sub")
		}
		userID = uint64(v)
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, errors.New("invalid sub")
		}
		userID = parsed
	default:
		return 0, errors.New("invalid sub")
	}

	expAny, ok := claims["exp"]
	if ok {
		switch exp := expAny.(type) {
		case float64:
			if time.Now().Unix() > int64(exp) {
				return 0, errors.New("expired")
			}
		case json.Number:
			n, _ := exp.Int64()
			if time.Now().Unix() > n {
				return 0, errors.New("expired")
			}
		}
	}
	return userID, nil
}

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

func (s *appState) encryptRconPassword(plaintext string) (string, error) {
	nonce := make([]byte, s.rconAead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	cipherText := s.rconAead.Seal(nil, nonce, []byte(plaintext), nil)
	buf := make([]byte, 0, len(nonce)+len(cipherText))
	buf = append(buf, nonce...)
	buf = append(buf, cipherText...)
	return base64.StdEncoding.EncodeToString(buf), nil
}

func (s *appState) decryptRconPassword(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := s.rconAead.NonceSize()
	if len(raw) <= nonceSize {
		return "", errors.New("invalid ciphertext")
	}
	nonce, cipherText := raw[:nonceSize], raw[nonceSize:]
	plain, err := s.rconAead.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func isMySQLDuplicate(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") || strings.Contains(strings.ToLower(msg), "duplicate entry")
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

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, defaultArgon.time, defaultArgon.memory, defaultArgon.threads, defaultArgon.keyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", defaultArgon.memory, defaultArgon.time, defaultArgon.threads, b64Salt, b64Hash), nil
}

func verifyPassword(password, encoded string) (bool, error) {
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

	if strings.HasPrefix(parts[2], "m=") || strings.Contains(parts[2], ",") {
	}
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
