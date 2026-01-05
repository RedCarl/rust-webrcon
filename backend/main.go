package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

type appState struct {
	store       *jsonStore
	jwtSecret   []byte
	rconAead    cipher.AEAD
	dockerBin   string
	authorLabel string
}

type apiErrorBody struct {
	Message string `json:"message"`
}

type appConfig struct {
	BindAddr          string `json:"bind_addr"`
	JWTSecret         string `json:"jwt_secret"`
	RconEncryptionKey string `json:"rcon_encryption_key"`
	DataDir           string `json:"data_dir"`
	DockerBin         string `json:"docker_bin"`
}

func main() {
	configPath := flag.String("config", "./config.json", "配置文件路径")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("读取配置文件失败：%v", err)
	}
	if err := cfg.validate(); err != nil {
		log.Fatalf("配置文件不合法：%v", err)
	}

	jwtSecret := []byte(cfg.JWTSecret)
	rconKey, err := parse32ByteKey(cfg.RconEncryptionKey)
	if err != nil {
		log.Fatalf("配置文件不合法：%v", err)
	}

	store, err := openJSONStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("初始化数据文件失败：%v", err)
	}
	if err := ensureDefaultUser(store); err != nil {
		log.Fatalf("初始化默认用户失败：%v", err)
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
		store:       store,
		jwtSecret:   jwtSecret,
		rconAead:    aead,
		dockerBin:   cfg.DockerBin,
		authorLabel: "CarlJoy",
	}

	addr := cfg.BindAddr
	log.Printf("[%s] 后端启动：%s", state.authorLabel, addr)

	router := gin.New()
	router.Use(gin.Logger())
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

		api.GET("/servers/:id/last-wipe", state.ginGetServerLastWipe)

		protected := api.Group("/")
		protected.Use(state.ginAuthRequired())
		{
			protected.GET("/me", state.ginMe)
			protected.GET("/servers", state.ginListServers)
			protected.POST("/servers", state.ginCreateServer)
			protected.PUT("/servers/:id", state.ginUpdateServer)
			protected.DELETE("/servers/:id", state.ginDeleteServer)
			protected.POST("/servers/:id/actions/:action", state.ginServerAction)

			admin := protected.Group("/admin")
			admin.Use(state.ginAdminRequired())
			{
				admin.GET("/users", state.ginAdminListUsers)
				admin.POST("/users", state.ginAdminCreateUser)
				admin.PUT("/users/:id/password", state.ginAdminUpdateUserPassword)
				admin.PUT("/users/:id/servers", state.ginAdminAssignUserServers)

				admin.GET("/servers", state.ginAdminListServers)
				admin.POST("/servers", state.ginAdminCreateServer)
				admin.PUT("/servers/:id", state.ginAdminUpdateServer)
				admin.DELETE("/servers/:id", state.ginAdminDeleteServer)
			}
		}
	}

	if err := router.Run(addr); err != nil {
		log.Fatalf("服务异常退出：%v", err)
	}
}

func ensureDefaultUser(store *jsonStore) error {
	if u, ok := store.FindUserByUsername("system"); ok {
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
		return store.UpdateUserPasswordHash("system", hash)
	}
	hash, err := hashPassword("123456")
	if err != nil {
		return err
	}
	if _, err := store.CreateUser("system", hash); err != nil {
		if errors.Is(err, errUserExists) {
			return nil
		}
		return err
	}
	return nil
}

func parse32ByteKey(raw string) ([]byte, error) {
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

func loadConfig(path string) (appConfig, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return appConfig{}, errors.New("配置文件路径为空")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return initConfigFile(p)
		}
		return appConfig{}, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return initConfigFile(p)
	}
	var cfg appConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return appConfig{}, err
	}
	cfg.normalize()
	return cfg, nil
}

func initConfigFile(path string) (appConfig, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return appConfig{}, err
		}
	}

	jwtSecret := make([]byte, 32)
	if _, err := rand.Read(jwtSecret); err != nil {
		return appConfig{}, err
	}

	rconKey := make([]byte, 32)
	if _, err := rand.Read(rconKey); err != nil {
		return appConfig{}, err
	}

	cfg := appConfig{
		BindAddr:          "0.0.0.0:8084",
		JWTSecret:         hex.EncodeToString(jwtSecret),
		RconEncryptionKey: base64.StdEncoding.EncodeToString(rconKey),
		DataDir:           ".",
		DockerBin:         "docker",
	}
	cfg.normalize()

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return appConfig{}, err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return appConfig{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return appConfig{}, err
	}
	return cfg, nil
}

func (c *appConfig) normalize() {
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

func (c *appConfig) validate() error {
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

	u, err := s.store.CreateUser(username, hash)
	if err != nil {
		if errors.Is(err, errUserExists) {
			c.JSON(409, apiErrorBody{Message: "用户名已存在"})
			return
		}
		if err.Error() == "invalid username" {
			c.JSON(400, apiErrorBody{Message: "用户名格式不合法"})
			return
		}
		c.JSON(500, apiErrorBody{Message: "创建用户失败"})
		return
	}
	c.JSON(200, registerResponse{ID: u.ID, Username: u.Username})
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
		c.JSON(400, apiErrorBody{Message: "用户名或密码不能为空"})
		return
	}

	u, ok := s.store.FindUserByUsername(username)
	if !ok {
		c.JSON(401, apiErrorBody{Message: "账号不存在"})
		return
	}

	passOK, err := verifyPassword(password, u.PasswordHash)
	if err != nil || !passOK {
		c.JSON(401, apiErrorBody{Message: "密码错误"})
		return
	}
	if strings.HasPrefix(u.PasswordHash, "argon2id$") {
		if next, err := hashPassword(password); err == nil {
			_ = s.store.UpdateUserPasswordHash(u.Username, next)
		}
	}

	token, err := signJWT(s.jwtSecret, u.ID)
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
	u, ok := s.store.FindUserByID(userID)
	if !ok {
		c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
		return
	}
	c.JSON(200, meResponse{ID: u.ID, Username: u.Username})
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
	servers := s.store.ListServersByUserID(userID)
	out := make([]serverDTO, 0, len(servers))
	for _, srv := range servers {
		pw, err := s.decryptRconPassword(srv.RconPasswordCipher)
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "查询服务器失败"})
			return
		}
		out = append(out, serverDTO{
			ID:            srv.ID,
			Name:          srv.Name,
			Host:          srv.Host,
			Port:          srv.Port,
			RconPassword:  pw,
			ContainerName: srv.ContainerName,
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

	srv, err := s.store.CreateServer([]uint64{userID}, name, host, req.Port, cipherText, containerName)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}

	c.JSON(200, serverDTO{
		ID:            srv.ID,
		Name:          srv.Name,
		Host:          srv.Host,
		Port:          srv.Port,
		RconPassword:  pw,
		ContainerName: srv.ContainerName,
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

	var req updateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}

	updated, err := s.store.UpdateServer(userID, id, func(srv *storedServer) error {
		if req.Name != nil {
			v := strings.TrimSpace(*req.Name)
			if v == "" {
				return errors.New("服务器名称不能为空")
			}
			srv.Name = v
		}
		if req.Host != nil {
			v := strings.TrimSpace(*req.Host)
			if v == "" {
				return errors.New("服务器地址不能为空")
			}
			srv.Host = v
		}
		if req.Port != nil {
			srv.Port = *req.Port
		}
		if req.RconPassword != nil {
			v := strings.TrimSpace(*req.RconPassword)
			if v == "" {
				return errors.New("RCON 密码不能为空")
			}
			cipherText, err := s.encryptRconPassword(v)
			if err != nil {
				return err
			}
			srv.RconPasswordCipher = cipherText
		}
		if req.ContainerName != nil {
			srv.ContainerName = normalizeOptionalString(req.ContainerName)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(404, apiErrorBody{Message: "资源不存在"})
			return
		}
		if err.Error() == "服务器名称不能为空" || err.Error() == "服务器地址不能为空" || err.Error() == "RCON 密码不能为空" {
			c.JSON(400, apiErrorBody{Message: err.Error()})
			return
		}
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}

	pw, err := s.decryptRconPassword(updated.RconPasswordCipher)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}

	c.JSON(200, serverDTO{
		ID:            id,
		Name:          updated.Name,
		Host:          updated.Host,
		Port:          updated.Port,
		RconPassword:  pw,
		ContainerName: updated.ContainerName,
	})
}

func (s *appState) ginDeleteServer(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	ok, err := s.store.DeleteServer(userID, id)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "删除服务器失败"})
		return
	}
	if !ok {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	c.Status(204)
}

type adminUserDTO struct {
	ID        uint64    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *appState) ginAdminListUsers(c *gin.Context) {
	list := s.store.ListUsers()
	out := make([]adminUserDTO, 0, len(list))
	for _, u := range list {
		out = append(out, adminUserDTO{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt})
	}
	c.JSON(200, out)
}

type adminCreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *appState) ginAdminCreateUser(c *gin.Context) {
	var req adminCreateUserRequest
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

	u, err := s.store.CreateUser(username, hash)
	if err != nil {
		if errors.Is(err, errUserExists) {
			c.JSON(409, apiErrorBody{Message: "用户名已存在"})
			return
		}
		if err.Error() == "invalid username" {
			c.JSON(400, apiErrorBody{Message: "用户名格式不合法"})
			return
		}
		c.JSON(500, apiErrorBody{Message: "创建用户失败"})
		return
	}
	c.JSON(200, adminUserDTO{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt})
}

type adminUpdateUserPasswordRequest struct {
	Password string `json:"password"`
}

func (s *appState) ginAdminUpdateUserPassword(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	u, ok := s.store.FindUserByID(id)
	if !ok {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	var req adminUpdateUserPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}
	password := strings.TrimSpace(req.Password)
	if len(password) < 6 {
		c.JSON(400, apiErrorBody{Message: "密码至少 6 位"})
		return
	}
	hash, err := hashPassword(password)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "生成密码哈希失败"})
		return
	}
	if err := s.store.UpdateUserPasswordHash(u.Username, hash); err != nil {
		c.JSON(500, apiErrorBody{Message: "修改失败"})
		return
	}
	c.Status(204)
}

type adminAssignUserServersRequest struct {
	ServerIDs []uint64 `json:"server_ids"`
}

func (s *appState) ginAdminAssignUserServers(c *gin.Context) {
	userID, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	if _, ok := s.store.FindUserByID(userID); !ok {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	systemUser, ok := s.store.FindUserByUsername("system")
	if !ok {
		c.JSON(500, apiErrorBody{Message: "系统账号不存在"})
		return
	}

	var req adminAssignUserServersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}

	want := make(map[uint64]struct{}, len(req.ServerIDs))
	for _, id := range req.ServerIDs {
		if id == 0 {
			continue
		}
		if _, exists := want[id]; exists {
			continue
		}
		if _, found := s.store.FindServerByIDAny(id); !found {
			c.JSON(400, apiErrorBody{Message: "服务器不存在"})
			return
		}
		want[id] = struct{}{}
	}

	current := s.store.ListServersByUserID(userID)
	for _, srv := range current {
		if _, keep := want[srv.ID]; keep {
			continue
		}
		_, err := s.store.UpdateServerAny(srv.ID, func(srv *storedServer) error {
			serverRemoveUser(srv, userID)
			if len(srv.UserIDs) == 0 {
				srv.UserIDs = []uint64{systemUser.ID}
			}
			srv.UserID = 0
			return nil
		})
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "分配失败"})
			return
		}
	}

	for srvID := range want {
		_, err := s.store.UpdateServerAny(srvID, func(srv *storedServer) error {
			if userID != systemUser.ID && len(srv.UserIDs) == 1 && srv.UserIDs[0] == systemUser.ID {
				srv.UserIDs = nil
			}
			serverAddUser(srv, userID)
			srv.UserID = 0
			return nil
		})
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "分配失败"})
			return
		}
	}

	c.Status(204)
}

type adminServerDTO struct {
	ID            uint64   `json:"id"`
	UserIDs       []uint64 `json:"user_ids"`
	Name          string   `json:"name"`
	Host          string   `json:"host"`
	Port          uint16   `json:"port"`
	RconPassword  string   `json:"rcon_password"`
	ContainerName *string  `json:"container_name"`
}

func (s *appState) ginAdminListServers(c *gin.Context) {
	servers := s.store.ListServersAny()
	out := make([]adminServerDTO, 0, len(servers))
	for _, srv := range servers {
		pw, err := s.decryptRconPassword(srv.RconPasswordCipher)
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "查询服务器失败"})
			return
		}
		out = append(out, adminServerDTO{
			ID:            srv.ID,
			UserIDs:       srv.UserIDs,
			Name:          srv.Name,
			Host:          srv.Host,
			Port:          srv.Port,
			RconPassword:  pw,
			ContainerName: srv.ContainerName,
		})
	}
	c.JSON(200, out)
}

type adminCreateServerRequest struct {
	UserID        uint64   `json:"user_id"`
	UserIDs       []uint64 `json:"user_ids"`
	Name          string   `json:"name"`
	Host          string   `json:"host"`
	Port          uint16   `json:"port"`
	RconPassword  string   `json:"rcon_password"`
	ContainerName *string  `json:"container_name"`
}

func (s *appState) ginAdminCreateServer(c *gin.Context) {
	var req adminCreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}
	userIDs := req.UserIDs
	if len(userIDs) == 0 && req.UserID != 0 {
		userIDs = []uint64{req.UserID}
	}
	userIDs = cleanUserIDs(userIDs)
	if len(userIDs) == 0 {
		c.JSON(400, apiErrorBody{Message: "所属账号不能为空"})
		return
	}
	for _, uid := range userIDs {
		if _, ok := s.store.FindUserByID(uid); !ok {
			c.JSON(400, apiErrorBody{Message: "所属账号不存在"})
			return
		}
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

	srv, err := s.store.CreateServer(userIDs, name, host, req.Port, cipherText, containerName)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}

	c.JSON(200, adminServerDTO{
		ID:            srv.ID,
		UserIDs:       srv.UserIDs,
		Name:          srv.Name,
		Host:          srv.Host,
		Port:          srv.Port,
		RconPassword:  pw,
		ContainerName: srv.ContainerName,
	})
}

type adminUpdateServerRequest struct {
	UserID        *uint64   `json:"user_id"`
	UserIDs       *[]uint64 `json:"user_ids"`
	Name          *string   `json:"name"`
	Host          *string   `json:"host"`
	Port          *uint16   `json:"port"`
	RconPassword  *string   `json:"rcon_password"`
	ContainerName *string   `json:"container_name"`
}

func (s *appState) ginAdminUpdateServer(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	var req adminUpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}

	updated, err := s.store.UpdateServerAny(id, func(srv *storedServer) error {
		if req.UserIDs != nil {
			userIDs := cleanUserIDs(*req.UserIDs)
			if len(userIDs) == 0 {
				return errors.New("所属账号不能为空")
			}
			for _, uid := range userIDs {
				if _, ok := s.store.FindUserByID(uid); !ok {
					return errors.New("所属账号不存在")
				}
			}
			srv.UserIDs = userIDs
			srv.UserID = 0
		} else if req.UserID != nil {
			if *req.UserID == 0 {
				return errors.New("所属账号不能为空")
			}
			if _, ok := s.store.FindUserByID(*req.UserID); !ok {
				return errors.New("所属账号不存在")
			}
			srv.UserIDs = []uint64{*req.UserID}
			srv.UserID = 0
		}
		if req.Name != nil {
			v := strings.TrimSpace(*req.Name)
			if v == "" {
				return errors.New("服务器名称不能为空")
			}
			srv.Name = v
		}
		if req.Host != nil {
			v := strings.TrimSpace(*req.Host)
			if v == "" {
				return errors.New("服务器地址不能为空")
			}
			srv.Host = v
		}
		if req.Port != nil {
			srv.Port = *req.Port
		}
		if req.RconPassword != nil {
			v := strings.TrimSpace(*req.RconPassword)
			if v == "" {
				return errors.New("RCON 密码不能为空")
			}
			cipherText, err := s.encryptRconPassword(v)
			if err != nil {
				return err
			}
			srv.RconPasswordCipher = cipherText
		}
		if req.ContainerName != nil {
			srv.ContainerName = normalizeOptionalString(req.ContainerName)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(404, apiErrorBody{Message: "资源不存在"})
			return
		}
		switch err.Error() {
		case "所属账号不能为空", "所属账号不存在", "服务器名称不能为空", "服务器地址不能为空", "RCON 密码不能为空":
			c.JSON(400, apiErrorBody{Message: err.Error()})
			return
		default:
			c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
			return
		}
	}

	pw, err := s.decryptRconPassword(updated.RconPasswordCipher)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}

	c.JSON(200, adminServerDTO{
		ID:            updated.ID,
		UserIDs:       updated.UserIDs,
		Name:          updated.Name,
		Host:          updated.Host,
		Port:          updated.Port,
		RconPassword:  pw,
		ContainerName: updated.ContainerName,
	})
}

func (s *appState) ginAdminDeleteServer(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	ok, err := s.store.DeleteServerAny(id)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "删除服务器失败"})
		return
	}
	if !ok {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	c.Status(204)
}

func (s *appState) ginGetServerLastWipe(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.Status(404)
		return
	}

	srv, found := s.store.FindServerByIDAny(id)
	if !found {
		c.Status(404)
		return
	}

	c.String(200, strings.TrimSpace(srv.LastWipeAt))
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

func (s *appState) ginAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := mustUserID(c)
		u, ok := s.store.FindUserByID(userID)
		if !ok || strings.TrimSpace(u.Username) != "system" {
			c.JSON(403, apiErrorBody{Message: "无权限"})
			c.Abort()
			return
		}
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

	srv, found := s.store.FindServerByID(userID, id)
	if !found {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	if srv.ContainerName == nil || strings.TrimSpace(*srv.ContainerName) == "" {
		c.JSON(400, apiErrorBody{Message: "该服务器未配置容器名称"})
		return
	}

	containerName := strings.TrimSpace(*srv.ContainerName)
	if !dockerNameRe.MatchString(containerName) {
		c.JSON(400, apiErrorBody{Message: "容器名称格式不合法"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()

	stdout, stderr, exitCode, err := dockerExecRustServer(ctx, s.dockerBin, containerName, dockerArg)
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

func dockerExecRustServer(ctx context.Context, dockerBin string, containerName string, rustServerArg string) (stdout string, stderr string, exitCode int, err error) {
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
