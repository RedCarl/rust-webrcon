package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"rust-panel-backend/internal/store"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerResponse struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

func (a *App) ginRegister(c *gin.Context) {
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

	u, err := a.Store.CreateUser(username, hash)
	if err != nil {
		if errors.Is(err, store.ErrUserExists) {
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

func (a *App) ginLogin(c *gin.Context) {
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

	u, ok := a.Store.FindUserByUsername(username)
	if !ok {
		c.JSON(401, apiErrorBody{Message: "账号不存在"})
		return
	}

	passOK, err := verifyPassword(password, u.Password)
	if err != nil || !passOK {
		c.JSON(401, apiErrorBody{Message: "密码错误"})
		return
	}
	if u.Password != password {
		_ = a.Store.UpdateUserPassword(u.Username, password)
	}
	token, err := signJWT(a.JWTSecret, u.ID)
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

func (a *App) ginMe(c *gin.Context) {
	userID := mustUserID(c)
	u, ok := a.Store.FindUserByID(userID)
	if !ok {
		c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
		return
	}
	c.JSON(200, meResponse{ID: u.ID, Username: u.Username})
}

type serverDTO struct {
	ID                     uint64  `json:"id"`
	Name                   string  `json:"name"`
	Host                   string  `json:"host"`
	Port                   uint16  `json:"port"`
	RconPassword           string  `json:"rcon_password"`
	ContainerName          *string `json:"container_name"`
	LastWipeAt             string  `json:"last_wipe_at"`
	AlwaysUseTodayWipeDate bool    `json:"always_use_today_wipe_date"`
}

type wipeConfigDTO struct {
	LgsmDir              string  `json:"lgsm_dir,omitempty"`
	CommonCfg            string  `json:"common_cfg,omitempty"`
	MapDir               string  `json:"map_dir,omitempty"`
	ServerIdentity       string  `json:"server_identity,omitempty"`
	RustMapsSearchURL    string  `json:"rustmaps_search_url,omitempty"`
	RustMapsSearchBody   string  `json:"rustmaps_search_body,omitempty"`
	S3Endpoint           string  `json:"s3_endpoint,omitempty"`
	S3Region             string  `json:"s3_region,omitempty"`
	S3Bucket             string  `json:"s3_bucket,omitempty"`
	S3AccessKey          string  `json:"s3_access_key,omitempty"`
	S3SecretKey          string  `json:"s3_secret_key,omitempty"`
	PublicBaseURL        string  `json:"public_base_url,omitempty"`
	WebRCONAPIURL        string  `json:"web_rcon_api_url,omitempty"`
	WebRCONServerID      *uint64 `json:"web_rcon_server_id,omitempty"`
	MapDetectTimeoutSecs int     `json:"map_detect_timeout_seconds,omitempty"`
}

func (a *App) ginListServers(c *gin.Context) {
	userID := mustUserID(c)
	servers := a.Store.ListServersByUserID(userID)
	out := make([]serverDTO, 0, len(servers))
	for _, srv := range servers {
		pw, err := a.readRconPassword(srv.RconPassword)
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "查询服务器失败"})
			return
		}
		out = append(out, serverDTO{
			ID:                     srv.ID,
			Name:                   srv.Name,
			Host:                   srv.Host,
			Port:                   srv.Port,
			RconPassword:           pw,
			ContainerName:          srv.ContainerName,
			LastWipeAt:             resolveServerLastWipeAt(srv),
			AlwaysUseTodayWipeDate: srv.AlwaysUseTodayWipeDate,
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

func (a *App) ginCreateServer(c *gin.Context) {
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
	storedText, err := a.storeRconPassword(pw)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}

	srv, err := a.Store.CreateServer([]uint64{userID}, name, host, req.Port, storedText, containerName, nil)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}

	c.JSON(200, serverDTO{
		ID:                     srv.ID,
		Name:                   srv.Name,
		Host:                   srv.Host,
		Port:                   srv.Port,
		RconPassword:           pw,
		ContainerName:          srv.ContainerName,
		LastWipeAt:             resolveServerLastWipeAt(srv),
		AlwaysUseTodayWipeDate: srv.AlwaysUseTodayWipeDate,
	})
}

type updateServerRequest struct {
	Name                   *string `json:"name"`
	Host                   *string `json:"host"`
	Port                   *uint16 `json:"port"`
	RconPassword           *string `json:"rcon_password"`
	ContainerName          *string `json:"container_name"`
	LastWipeAt             *string `json:"last_wipe_at"`
	AlwaysUseTodayWipeDate *bool   `json:"always_use_today_wipe_date"`
}

func (a *App) ginUpdateServer(c *gin.Context) {
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

	updated, err := a.Store.UpdateServer(userID, id, func(srv *store.Server) error {
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
			storedText, err := a.storeRconPassword(v)
			if err != nil {
				return err
			}
			srv.RconPassword = storedText
		}
		if req.ContainerName != nil {
			srv.ContainerName = normalizeOptionalString(req.ContainerName)
		}
		if req.LastWipeAt != nil {
			srv.LastWipeAt = strings.TrimSpace(*req.LastWipeAt)
		}
		if req.AlwaysUseTodayWipeDate != nil {
			srv.AlwaysUseTodayWipeDate = *req.AlwaysUseTodayWipeDate
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

	pw, err := a.readRconPassword(updated.RconPassword)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}

	c.JSON(200, serverDTO{
		ID:                     id,
		Name:                   updated.Name,
		Host:                   updated.Host,
		Port:                   updated.Port,
		RconPassword:           pw,
		ContainerName:          updated.ContainerName,
		LastWipeAt:             resolveServerLastWipeAt(updated),
		AlwaysUseTodayWipeDate: updated.AlwaysUseTodayWipeDate,
	})
}

func (a *App) ginDeleteServer(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	ok, err := a.Store.DeleteServer(userID, id)
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

func (a *App) ginAdminListUsers(c *gin.Context) {
	list := a.Store.ListUsers()
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

func (a *App) ginAdminCreateUser(c *gin.Context) {
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

	u, err := a.Store.CreateUser(username, hash)
	if err != nil {
		if errors.Is(err, store.ErrUserExists) {
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

func (a *App) ginAdminUpdateUserPassword(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	u, ok := a.Store.FindUserByID(id)
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
	if err := a.Store.UpdateUserPassword(u.Username, hash); err != nil {
		c.JSON(500, apiErrorBody{Message: "修改失败"})
		return
	}
	c.Status(204)
}

type adminAssignUserServersRequest struct {
	ServerIDs []uint64 `json:"server_ids"`
}

func (a *App) ginAdminAssignUserServers(c *gin.Context) {
	userID, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	if _, ok := a.Store.FindUserByID(userID); !ok {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	systemUser, ok := a.Store.FindUserByUsername("system")
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
		if _, found := a.Store.FindServerByIDAny(id); !found {
			c.JSON(400, apiErrorBody{Message: "服务器不存在"})
			return
		}
		want[id] = struct{}{}
	}

	current := a.Store.ListServersByUserID(userID)
	for _, srv := range current {
		if _, keep := want[srv.ID]; keep {
			continue
		}
		_, err := a.Store.UpdateServerAny(srv.ID, func(srv *store.Server) error {
			srv.UserIDs = removeUserID(srv.UserIDs, userID)
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
		_, err := a.Store.UpdateServerAny(srvID, func(srv *store.Server) error {
			if userID != systemUser.ID && len(srv.UserIDs) == 1 && srv.UserIDs[0] == systemUser.ID {
				srv.UserIDs = nil
			}
			srv.UserIDs = addUserID(srv.UserIDs, userID)
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
	ID                     uint64   `json:"id"`
	UserIDs                []uint64 `json:"user_ids"`
	Name                   string   `json:"name"`
	Host                   string   `json:"host"`
	Port                   uint16   `json:"port"`
	RconPassword           string   `json:"rcon_password"`
	ContainerName          *string  `json:"container_name"`
	LastWipeAt             string   `json:"last_wipe_at"`
	AlwaysUseTodayWipeDate bool     `json:"always_use_today_wipe_date"`
}

func (a *App) ginAdminListServers(c *gin.Context) {
	servers := a.Store.ListServersAny()
	out := make([]adminServerDTO, 0, len(servers))
	for _, srv := range servers {
		pw, err := a.readRconPassword(srv.RconPassword)
		if err != nil {
			c.JSON(500, apiErrorBody{Message: "查询服务器失败"})
			return
		}
		out = append(out, adminServerDTO{
			ID:                     srv.ID,
			UserIDs:                srv.UserIDs,
			Name:                   srv.Name,
			Host:                   srv.Host,
			Port:                   srv.Port,
			RconPassword:           pw,
			ContainerName:          srv.ContainerName,
			LastWipeAt:             resolveServerLastWipeAt(srv),
			AlwaysUseTodayWipeDate: srv.AlwaysUseTodayWipeDate,
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

func (a *App) ginAdminCreateServer(c *gin.Context) {
	var req adminCreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}
	userIDs := req.UserIDs
	if len(userIDs) == 0 && req.UserID != 0 {
		userIDs = []uint64{req.UserID}
	}
	userIDs = store.CleanUserIDs(userIDs)
	if len(userIDs) == 0 {
		c.JSON(400, apiErrorBody{Message: "所属账号不能为空"})
		return
	}
	for _, uid := range userIDs {
		if _, ok := a.Store.FindUserByID(uid); !ok {
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
	storedText, err := a.storeRconPassword(pw)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}

	srv, err := a.Store.CreateServer(userIDs, name, host, req.Port, storedText, containerName, nil)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "创建服务器失败"})
		return
	}

	c.JSON(200, adminServerDTO{
		ID:                     srv.ID,
		UserIDs:                srv.UserIDs,
		Name:                   srv.Name,
		Host:                   srv.Host,
		Port:                   srv.Port,
		RconPassword:           pw,
		ContainerName:          srv.ContainerName,
		LastWipeAt:             resolveServerLastWipeAt(srv),
		AlwaysUseTodayWipeDate: srv.AlwaysUseTodayWipeDate,
	})
}

type adminUpdateServerRequest struct {
	UserID                 *uint64   `json:"user_id"`
	UserIDs                *[]uint64 `json:"user_ids"`
	Name                   *string   `json:"name"`
	Host                   *string   `json:"host"`
	Port                   *uint16   `json:"port"`
	RconPassword           *string   `json:"rcon_password"`
	ContainerName          *string   `json:"container_name"`
	LastWipeAt             *string   `json:"last_wipe_at"`
	AlwaysUseTodayWipeDate *bool     `json:"always_use_today_wipe_date"`
}

func wipeConfigToDTO(cfg *store.WipeConfig) *wipeConfigDTO {
	if cfg == nil {
		return nil
	}
	return &wipeConfigDTO{
		LgsmDir:              cfg.LGSMDir,
		CommonCfg:            cfg.CommonCfg,
		MapDir:               cfg.MapDir,
		ServerIdentity:       cfg.ServerIdentity,
		RustMapsSearchURL:    cfg.RustMapsSearchURL,
		RustMapsSearchBody:   cfg.RustMapsSearchBody,
		S3Endpoint:           cfg.S3Endpoint,
		S3Region:             cfg.S3Region,
		S3Bucket:             cfg.S3Bucket,
		S3AccessKey:          cfg.S3AccessKey,
		PublicBaseURL:        cfg.PublicBaseURL,
		WebRCONAPIURL:        cfg.WebRCONAPIURL,
		WebRCONServerID:      cfg.WebRCONServerID,
		MapDetectTimeoutSecs: cfg.MapDetectTimeoutSecs,
	}
}

func (a *App) wipeConfigFromDTO(input *wipeConfigDTO, current *store.WipeConfig) (*store.WipeConfig, error) {
	if input == nil {
		return current, nil
	}
	next := &store.WipeConfig{}
	if current != nil {
		*next = *current
	}
	next.LGSMDir = strings.TrimSpace(input.LgsmDir)
	next.CommonCfg = strings.TrimSpace(input.CommonCfg)
	next.MapDir = strings.TrimSpace(input.MapDir)
	next.ServerIdentity = strings.TrimSpace(input.ServerIdentity)
	next.RustMapsSearchURL = strings.TrimSpace(input.RustMapsSearchURL)
	next.RustMapsSearchBody = strings.TrimSpace(input.RustMapsSearchBody)
	next.S3Endpoint = strings.TrimSpace(input.S3Endpoint)
	next.S3Region = strings.TrimSpace(input.S3Region)
	next.S3Bucket = strings.TrimSpace(input.S3Bucket)
	next.S3AccessKey = strings.TrimSpace(input.S3AccessKey)
	next.PublicBaseURL = strings.TrimSpace(input.PublicBaseURL)
	next.WebRCONAPIURL = strings.TrimSpace(input.WebRCONAPIURL)
	next.MapDetectTimeoutSecs = input.MapDetectTimeoutSecs
	if input.WebRCONServerID != nil {
		v := *input.WebRCONServerID
		next.WebRCONServerID = &v
	}
	if input.S3SecretKey != "" {
		storedText, err := a.storeRconPassword(strings.TrimSpace(input.S3SecretKey))
		if err != nil {
			return nil, errors.New("S3 Secret 保存失败")
		}
		next.S3SecretKey = storedText
	}
	return next, nil
}

type adminWipeSettingsResponse struct {
	WipeConfig *wipeConfigDTO `json:"wipe_config,omitempty"`
}

type adminUpdateWipeSettingsRequest struct {
	WipeConfig *wipeConfigDTO `json:"wipe_config"`
}

func (a *App) ginAdminGetWipeSettings(c *gin.Context) {
	settings := a.Store.GetGlobalSettings()
	c.JSON(200, adminWipeSettingsResponse{
		WipeConfig: wipeConfigToDTO(settings.WipeConfig),
	})
}

func (a *App) ginAdminUpdateWipeSettings(c *gin.Context) {
	var req adminUpdateWipeSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}
	next, err := a.wipeConfigFromDTO(req.WipeConfig, a.Store.GetGlobalSettings().WipeConfig)
	if err != nil {
		c.JSON(400, apiErrorBody{Message: err.Error()})
		return
	}
	updated, err := a.Store.UpdateGlobalSettings(func(settings *store.GlobalSettings) error {
		settings.WipeConfig = next
		return nil
	})
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "更新清档设置失败"})
		return
	}
	c.JSON(200, adminWipeSettingsResponse{
		WipeConfig: wipeConfigToDTO(updated.WipeConfig),
	})
}

func (a *App) ginAdminUpdateServer(c *gin.Context) {
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

	updated, err := a.Store.UpdateServerAny(id, func(srv *store.Server) error {
		if req.UserIDs != nil {
			userIDs := store.CleanUserIDs(*req.UserIDs)
			if len(userIDs) == 0 {
				return errors.New("所属账号不能为空")
			}
			for _, uid := range userIDs {
				if _, ok := a.Store.FindUserByID(uid); !ok {
					return errors.New("所属账号不存在")
				}
			}
			srv.UserIDs = userIDs
			srv.UserID = 0
		} else if req.UserID != nil {
			if *req.UserID == 0 {
				return errors.New("所属账号不能为空")
			}
			if _, ok := a.Store.FindUserByID(*req.UserID); !ok {
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
			storedText, err := a.storeRconPassword(v)
			if err != nil {
				return err
			}
			srv.RconPassword = storedText
		}
		if req.ContainerName != nil {
			srv.ContainerName = normalizeOptionalString(req.ContainerName)
		}
		if req.LastWipeAt != nil {
			srv.LastWipeAt = strings.TrimSpace(*req.LastWipeAt)
		}
		if req.AlwaysUseTodayWipeDate != nil {
			srv.AlwaysUseTodayWipeDate = *req.AlwaysUseTodayWipeDate
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

	pw, err := a.readRconPassword(updated.RconPassword)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "更新服务器失败"})
		return
	}

	c.JSON(200, adminServerDTO{
		ID:                     updated.ID,
		UserIDs:                updated.UserIDs,
		Name:                   updated.Name,
		Host:                   updated.Host,
		Port:                   updated.Port,
		RconPassword:           pw,
		ContainerName:          updated.ContainerName,
		LastWipeAt:             resolveServerLastWipeAt(updated),
		AlwaysUseTodayWipeDate: updated.AlwaysUseTodayWipeDate,
	})
}

func (a *App) ginAdminDeleteServer(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}
	ok, err := a.Store.DeleteServerAny(id)
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

func (a *App) ginGetServerLastWipe(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.Status(404)
		return
	}

	srv, found := a.Store.FindServerByIDAny(id)
	if !found {
		c.Status(404)
		return
	}

	c.String(200, resolveServerLastWipeAt(*srv))
}

func resolveServerLastWipeAt(srv store.Server) string {
	text := strings.TrimSpace(srv.LastWipeAt)
	if !srv.AlwaysUseTodayWipeDate {
		return text
	}

	now := time.Now()
	base := now
	if parsed, ok := parseServerLastWipeAt(text, now.Location()); ok {
		base = parsed
	}

	return time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		base.Hour(),
		base.Minute(),
		base.Second(),
		base.Nanosecond(),
		now.Location(),
	).Format("2006-01-02T15:04:05")
}

func parseServerLastWipeAt(value string, loc *time.Location) (time.Time, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, text, loc)
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (a *App) ginServerAction(c *gin.Context) {
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

	srv, found := a.Store.FindServerByID(userID, id)
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

	ctx, cancel := newActionTimeout(c.Request.Context())
	defer cancel()

	stdout, stderr, exitCode, err := dockerExecRustServer(ctx, a.DockerBin, containerName, dockerArg)
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

type serverCommandRequest struct {
	Command  string `json:"command"`
	Password string `json:"password"`
}

type serverCommandResponse struct {
	Result string `json:"result"`
}

func (a *App) ginServerCommand(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	var req serverCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}

	if strings.TrimSpace(req.Command) == "" {
		c.JSON(400, apiErrorBody{Message: "命令不能为空"})
		return
	}
	if req.Password == "" {
		c.JSON(401, apiErrorBody{Message: "RCON 密码不能为空"})
		return
	}

	srv, found := a.Store.FindServerByIDAny(id)
	if !found {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	// 校验 RCON 密码
	storedPassword, err := a.readRconPassword(srv.RconPassword)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "服务器配置错误"})
		return
	}
	if req.Password != storedPassword {
		c.JSON(401, apiErrorBody{Message: "RCON 密码错误"})
		return
	}

	result, err := a.sendRconCommand(srv.Host, srv.Port, storedPassword, req.Command)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: fmt.Sprintf("执行失败: %v", err)})
		return
	}

	c.JSON(200, serverCommandResponse{Result: result})
}
