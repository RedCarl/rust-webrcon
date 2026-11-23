package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"rust-panel-backend/internal/store"

	"github.com/gin-gonic/gin"
)

const (
	defaultRustMapsSearchURL = "https://api.rustmaps.com/internal/v1/maps/search"
	defaultWebRCONAPIURL     = "https://rust-webrcon.carljoy.cn"
	defaultLGSMDir           = "/data/serverfiles"
	defaultCommonCfg         = "/data/config-lgsm/rustserver/common.cfg"
	defaultMapDir            = "/data/serverfiles/server/rustserver"
	defaultDetectTimeoutSecs = 300
)

type wipeExecuteRequest struct {
	Seed *int64 `json:"seed"`
	Size *int   `json:"size"`
}

type wipeExecuteResponse struct {
	OK                  bool              `json:"ok"`
	PartialSuccess      bool              `json:"partial_success"`
	Stage               string            `json:"stage"`
	Message             string            `json:"message"`
	Logs                []string          `json:"logs"`
	SelectedSeed        int64             `json:"selected_seed,omitempty"`
	SelectedSize        int               `json:"selected_size,omitempty"`
	MapFilename         string            `json:"map_filename,omitempty"`
	MapS3Key            string            `json:"map_s3_key,omitempty"`
	MapURL              string            `json:"map_url,omitempty"`
	WebRCONDelivered    bool              `json:"web_rcon_delivered"`
	WebRCONErrorMessage string            `json:"web_rcon_error_message,omitempty"`
	Details             map[string]string `json:"details,omitempty"`
}

type wipeRuntimeConfig struct {
	ServerID           uint64
	LGSMDir            string
	CommonCfg          string
	MapDir             string
	ServerIdentity     string
	RustMapsSearchURL  string
	RustMapsSearchBody string
	S3Endpoint         string
	S3Region           string
	S3Bucket           string
	S3AccessKey        string
	S3SecretKey        string
	PublicBaseURL      string
	WebRCONAPIURL      string
	WebRCONServerID    uint64
	MapDetectTimeout   time.Duration
	DockerBin          string
	ContainerName      string
	DecryptedRCONPass  string
	LastSuccessfulSeed string
	StateFilePath      string
}

type rustMapsSearchResponse struct {
	Data []struct {
		Seed json.Number `json:"seed"`
		Size json.Number `json:"size"`
	} `json:"data"`
}

type seedCandidate struct {
	Seed int64
	Size int
}

type mapSnapshot struct {
	Size    int64
	ModTime time.Time
}

type detectedMap struct {
	Filename string
	Path     string
	Size     int64
	ModTime  time.Time
}

type wipeStateFile struct {
	MapFilename  string `json:"MAP_FILENAME"`
	MapLocalPath string `json:"MAP_LOCAL_PATH"`
	MapS3Key     string `json:"MAP_S3_KEY"`
	MapURL       string `json:"MAP_URL"`
	MapSeed      int64  `json:"MAP_SEED"`
	MapSize      int    `json:"MAP_SIZE"`
	GeneratedAt  string `json:"GENERATED_AT"`
}

func (a *App) ginExecuteServerWipe(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	var req wipeExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(400, apiErrorBody{Message: "请求格式错误"})
		return
	}

	srv, found := a.Store.FindServerByID(userID, id)
	if !found {
		c.JSON(404, apiErrorBody{Message: "资源不存在"})
		return
	}

	cfg, err := a.buildWipeRuntimeConfig(srv)
	if err != nil {
		c.JSON(400, apiErrorBody{Message: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Minute)
	defer cancel()

	result, execErr := a.executeFullWipe(ctx, id, cfg, req)
	if execErr != nil {
		status := 500
		if errors.Is(execErr, errInvalidWipeConfig) {
			status = 400
		}
		c.JSON(status, result)
		return
	}

	c.JSON(200, result)
}

var errInvalidWipeConfig = errors.New("invalid wipe config")

func (a *App) buildWipeRuntimeConfig(srv *store.Server) (wipeRuntimeConfig, error) {
	settings := a.Store.GetGlobalSettings()
	if settings.WipeConfig == nil {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: 未配置全局清档设置", errInvalidWipeConfig)
	}
	globalCfg := settings.WipeConfig
	containerName := ""
	if srv.ContainerName != nil {
		containerName = strings.TrimSpace(*srv.ContainerName)
	}

	rconPassword, err := a.readRconPassword(srv.RconPassword)
	if err != nil {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: RCON 密码读取失败", errInvalidWipeConfig)
	}

	cfg := wipeRuntimeConfig{
		ServerID:           srv.ID,
		LGSMDir:            strings.TrimSpace(globalCfg.LGSMDir),
		CommonCfg:          strings.TrimSpace(globalCfg.CommonCfg),
		MapDir:             strings.TrimSpace(globalCfg.MapDir),
		ServerIdentity:     strings.TrimSpace(globalCfg.ServerIdentity),
		RustMapsSearchURL:  strings.TrimSpace(globalCfg.RustMapsSearchURL),
		RustMapsSearchBody: strings.TrimSpace(globalCfg.RustMapsSearchBody),
		S3Endpoint:         strings.TrimSpace(globalCfg.S3Endpoint),
		S3Region:           strings.TrimSpace(globalCfg.S3Region),
		S3Bucket:           strings.TrimSpace(globalCfg.S3Bucket),
		S3AccessKey:        strings.TrimSpace(globalCfg.S3AccessKey),
		PublicBaseURL:      strings.TrimSpace(globalCfg.PublicBaseURL),
		WebRCONAPIURL:      strings.TrimSpace(globalCfg.WebRCONAPIURL),
		DockerBin:          a.DockerBin,
		ContainerName:      containerName,
		DecryptedRCONPass:  rconPassword,
	}

	if globalCfg.WebRCONServerID != nil {
		cfg.WebRCONServerID = *globalCfg.WebRCONServerID
	}
	if text := strings.TrimSpace(globalCfg.S3SecretKey); text != "" {
		secret, decErr := a.readRconPassword(text)
		if decErr != nil {
			return wipeRuntimeConfig{}, fmt.Errorf("%w: S3 Secret 读取失败", errInvalidWipeConfig)
		}
		cfg.S3SecretKey = secret
	}

	if containerName == "" {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: 未配置容器名称", errInvalidWipeConfig)
	}
	if !dockerNameRe.MatchString(containerName) {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: 容器名称格式不合法", errInvalidWipeConfig)
	}
	if cfg.LGSMDir == "" {
		cfg.LGSMDir = defaultLGSMDir
	}
	if cfg.CommonCfg == "" {
		cfg.CommonCfg = defaultCommonCfg
	}
	if cfg.MapDir == "" {
		cfg.MapDir = defaultMapDir
	}
	if cfg.RustMapsSearchURL == "" {
		cfg.RustMapsSearchURL = defaultRustMapsSearchURL
	}
	if cfg.WebRCONAPIURL == "" {
		cfg.WebRCONAPIURL = defaultWebRCONAPIURL
	}
	if cfg.ServerIdentity == "" {
		cfg.ServerIdentity = containerName
	}
	if cfg.RustMapsSearchBody == "" {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: 缺少 rustmaps_search_body", errInvalidWipeConfig)
	}
	if cfg.S3Endpoint == "" || cfg.S3Bucket == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: S3 配置不完整", errInvalidWipeConfig)
	}
	if cfg.PublicBaseURL == "" {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: public_base_url 不能为空", errInvalidWipeConfig)
	}
	if !isPublicHTTPURL(cfg.PublicBaseURL) {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: public_base_url 必须是公网 HTTP/HTTPS 地址", errInvalidWipeConfig)
	}
	if cfg.WebRCONServerID == 0 {
		return wipeRuntimeConfig{}, fmt.Errorf("%w: web_rcon_server_id 不能为空", errInvalidWipeConfig)
	}
	timeoutSecs := globalCfg.MapDetectTimeoutSecs
	if timeoutSecs <= 0 {
		timeoutSecs = defaultDetectTimeoutSecs
	}
	cfg.MapDetectTimeout = time.Duration(timeoutSecs) * time.Second
	cfg.StateFilePath = filepath.Join(a.DataDir, "wipe-state", fmt.Sprintf("%d.json", srv.ID))
	if state, err := readLastWipeState(cfg.StateFilePath); err == nil {
		cfg.LastSuccessfulSeed = strings.TrimSpace(strconv.FormatInt(state.MapSeed, 10))
	}

	return cfg, nil
}

func (a *App) executeFullWipe(ctx context.Context, serverID uint64, cfg wipeRuntimeConfig, req wipeExecuteRequest) (wipeExecuteResponse, error) {
	logger := newWipeLogger()
	result := wipeExecuteResponse{
		Stage:   "init",
		Message: "开始执行清档",
		Logs:    logger.Logs(),
	}

	logger.Printf("阶段 1/12: 从 RustMaps 获取候选种子")
	seed, size, err := a.pickSeed(ctx, cfg, req, logger)
	if err != nil {
		result.Stage = "rustmaps"
		result.Message = "获取种子失败"
		result.Logs = logger.Logs()
		return result, err
	}
	result.SelectedSeed = seed
	result.SelectedSize = size
	logger.Printf("本次种子: %d, 尺寸: %d", seed, size)

	logger.Printf("阶段 2/12: 快照地图目录")
	beforeSnapshot, err := snapshotMapDir(ctx, cfg, logger)
	if err != nil {
		result.Stage = "snapshot"
		result.Message = "地图目录快照失败"
		result.Logs = logger.Logs()
		return result, err
	}

	logger.Printf("阶段 3/12: 写入 common.cfg 的 seed/worldsize")
	if err := writeSeedAndSize(ctx, cfg, seed, size, logger); err != nil {
		result.Stage = "write_common_cfg"
		result.Message = "写入 seed/worldsize 失败"
		result.Logs = logger.Logs()
		return result, err
	}

	logger.Printf("阶段 4/12: 执行 LinuxGSM full wipe")
	fwStdout, fwStderr, fwExitCode, err := runFullWipe(ctx, cfg, logger)
	if err != nil {
		result.Stage = "full_wipe"
		result.Message = "执行 ./rustserver fw 失败"
		result.Logs = logger.Logs()
		result.Details = map[string]string{
			"stdout":    fwStdout,
			"stderr":    fwStderr,
			"exit_code": strconv.Itoa(fwExitCode),
		}
		return result, err
	}

	logger.Printf("阶段 5/12: 等待新地图生成")
	mapInfo, err := waitForFreshMap(ctx, cfg, beforeSnapshot, logger)
	if err != nil {
		result.Stage = "detect_map"
		result.Message = "未识别到新地图"
		result.Logs = logger.Logs()
		return result, err
	}
	result.MapFilename = mapInfo.Filename
	logger.Printf("识别到地图文件: %s", mapInfo.Filename)

	mapKey := pathJoinURL("rust/maps", cfg.ServerIdentity, strconv.FormatUint(cfg.ServerID, 10), mapInfo.Filename)
	logger.Printf("阶段 6/12: 上传地图到 S3")
	if err := uploadMapToS3(ctx, cfg, mapInfo, mapKey, logger); err != nil {
		result.Stage = "upload_s3"
		result.Message = "上传地图失败"
		result.Logs = logger.Logs()
		return result, err
	}
	result.MapS3Key = mapKey

	logger.Printf("阶段 7/12: 生成 map_url")
	mapURL, err := buildMapURL(cfg.PublicBaseURL, mapKey)
	if err != nil {
		result.Stage = "build_map_url"
		result.Message = "生成 map_url 失败"
		result.Logs = logger.Logs()
		return result, err
	}
	result.MapURL = mapURL

	logger.Printf("阶段 8/12: 写入 common.cfg 的 levelurl")
	if err := writeLevelURL(ctx, cfg, mapURL, logger); err != nil {
		result.Stage = "write_levelurl"
		result.Message = "写入 levelurl 失败"
		result.Logs = logger.Logs()
		return result, err
	}

	logger.Printf("阶段 9/12: 运行时下发 server.levelurl")
	webRCONErr := postWebRCONLevelURL(ctx, cfg, mapURL, logger)
	if webRCONErr != nil {
		result.PartialSuccess = true
		result.WebRCONErrorMessage = webRCONErr.Error()
		logger.Printf("WebRCON 下发失败: %v", webRCONErr)
	} else {
		result.WebRCONDelivered = true
	}

	logger.Printf("阶段 10/12: 写入状态文件")
	generatedAt := time.Now().Format(time.RFC3339)
	if err := writeWipeState(cfg.StateFilePath, wipeStateFile{
		MapFilename:  mapInfo.Filename,
		MapLocalPath: mapInfo.Path,
		MapS3Key:     mapKey,
		MapURL:       mapURL,
		MapSeed:      seed,
		MapSize:      size,
		GeneratedAt:  generatedAt,
	}); err != nil {
		result.Stage = "write_state"
		result.Message = "写入状态文件失败"
		result.Logs = logger.Logs()
		return result, err
	}

	logger.Printf("阶段 11/12: 更新服务器清档时间")
	if _, err := a.Store.UpdateServerAny(serverID, func(srv *store.Server) error {
		srv.LastWipeAt = generatedAt
		return nil
	}); err != nil {
		result.Stage = "update_server"
		result.Message = "更新服务器状态失败"
		result.Logs = logger.Logs()
		return result, err
	}

	logger.Printf("阶段 12/12: 清档流程完成")
	result.OK = true
	result.Stage = "done"
	if result.PartialSuccess {
		result.Message = "清档成功，但运行时下发 levelurl 失败"
	} else {
		result.Message = "清档成功"
	}
	result.Logs = logger.Logs()
	return result, nil
}

type wipeLogger struct {
	logs []string
}

func newWipeLogger() *wipeLogger {
	return &wipeLogger{logs: make([]string, 0, 32)}
}

func (l *wipeLogger) Printf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	l.logs = append(l.logs, fmt.Sprintf("%s %s", time.Now().Format("2006-01-02 15:04:05"), line))
	log.Print(line)
}

func (l *wipeLogger) Logs() []string {
	out := make([]string, len(l.logs))
	copy(out, l.logs)
	return out
}

func (a *App) pickSeed(ctx context.Context, cfg wipeRuntimeConfig, req wipeExecuteRequest, logger *wipeLogger) (int64, int, error) {
	if req.Seed != nil && req.Size != nil {
		logger.Printf("使用手动指定种子: %d, 尺寸: %d", *req.Seed, *req.Size)
		return *req.Seed, *req.Size, nil
	}

	body := strings.TrimSpace(cfg.RustMapsSearchBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.RustMapsSearchURL, strings.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, 0, fmt.Errorf("RustMaps 响应失败: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}

	var payload rustMapsSearchResponse
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return 0, 0, err
	}

	candidates := make([]seedCandidate, 0, len(payload.Data))
	lastSeed := strings.TrimSpace(cfg.LastSuccessfulSeed)
	for _, item := range payload.Data {
		seed, err := item.Seed.Int64()
		if err != nil {
			continue
		}
		size64, err := item.Size.Int64()
		if err != nil {
			continue
		}
		if size64 <= 0 {
			continue
		}
		candidate := seedCandidate{Seed: seed, Size: int(size64)}
		if lastSeed != "" && lastSeed == strconv.FormatInt(seed, 10) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		for _, item := range payload.Data {
			seed, err := item.Seed.Int64()
			if err != nil {
				continue
			}
			size64, err := item.Size.Int64()
			if err != nil || size64 <= 0 {
				continue
			}
			candidates = append(candidates, seedCandidate{Seed: seed, Size: int(size64)})
		}
	}
	if len(candidates) == 0 {
		return 0, 0, errors.New("RustMaps 没有返回可用候选种子")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	selected := candidates[rng.Intn(len(candidates))]
	return selected.Seed, selected.Size, nil
}

func snapshotMapDir(ctx context.Context, cfg wipeRuntimeConfig, logger *wipeLogger) (map[string]mapSnapshot, error) {
	_ = ctx
	current, err := scanMapDir(cfg.MapDir)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string]mapSnapshot, len(current))
	for filename, item := range current {
		snapshot[filename] = mapSnapshot{
			Size:    item.Size,
			ModTime: item.ModTime,
		}
	}
	return snapshot, nil
}

func waitForFreshMap(ctx context.Context, cfg wipeRuntimeConfig, before map[string]mapSnapshot, logger *wipeLogger) (detectedMap, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, cfg.MapDetectTimeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		current, err := scanMapDir(cfg.MapDir)
		if err == nil {
			if found, ok := detectFreshMap(current, before); ok {
				return found, nil
			}
		} else {
			logger.Printf("地图目录扫描失败: %v", err)
		}

		select {
		case <-timeoutCtx.Done():
			return detectedMap{}, errors.New("等待新地图超时")
		case <-ticker.C:
		}
	}
}

func detectFreshMap(current map[string]detectedMap, before map[string]mapSnapshot) (detectedMap, bool) {
	candidates := make([]detectedMap, 0)
	for filename, item := range current {
		if item.Size <= 0 {
			continue
		}
		cur := mapSnapshot{Size: item.Size, ModTime: item.ModTime}
		prev, existed := before[filename]
		if existed && prev.Size == cur.Size && prev.ModTime.Equal(cur.ModTime) {
			continue
		}
		candidates = append(candidates, item)
	}
	if len(candidates) == 0 {
		return detectedMap{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ModTime.After(candidates[j].ModTime)
	})
	return candidates[0], true
}

func writeSeedAndSize(ctx context.Context, cfg wipeRuntimeConfig, seed int64, size int, logger *wipeLogger) error {
	_ = ctx
	return updateCommonConfig(cfg.CommonCfg, map[string]string{
		"seed":      strconv.FormatInt(seed, 10),
		"worldsize": strconv.Itoa(size),
	})
}

func writeLevelURL(ctx context.Context, cfg wipeRuntimeConfig, mapURL string, logger *wipeLogger) error {
	_ = ctx
	_ = logger
	return updateCommonConfig(cfg.CommonCfg, map[string]string{
		"levelurl": mapURL,
	})
}

func runFullWipe(ctx context.Context, cfg wipeRuntimeConfig, logger *wipeLogger) (string, string, int, error) {
	logger.Printf("通过 Docker 容器内的 LinuxGSM 执行 ./rustserver fw")
	return dockerExecRustServerInDir(ctx, cfg.DockerBin, cfg.ContainerName, cfg.LGSMDir, "fw")
}

func uploadMapToS3(ctx context.Context, cfg wipeRuntimeConfig, mapInfo detectedMap, mapKey string, logger *wipeLogger) error {
	encodedSecret := base64.StdEncoding.EncodeToString([]byte(cfg.S3SecretKey))
	cmd := fmt.Sprintf("python3 - <<'PY'\n%s\nPY", buildS3PythonScript(cfg, mapInfo.Path, mapKey, encodedSecret))
	stdout, stderr, exitCode, err := runHostShellCommand(ctx, cmd)
	if err != nil {
		logger.Printf("上传到 S3 失败, exit=%d stdout=%s stderr=%s", exitCode, strings.TrimSpace(stdout), strings.TrimSpace(stderr))
		return err
	}
	if strings.TrimSpace(stdout) != "" {
		logger.Printf("S3 上传输出: %s", strings.TrimSpace(stdout))
	}
	return nil
}

func buildS3PythonScript(cfg wipeRuntimeConfig, localPath string, mapKey string, encodedSecret string) string {
	var script strings.Builder
	script.WriteString("import base64\n")
	script.WriteString("import hashlib\n")
	script.WriteString("import hmac\n")
	script.WriteString("import os\n")
	script.WriteString("import sys\n")
	script.WriteString("import urllib.parse\n")
	script.WriteString("import urllib.request\n")
	script.WriteString("import urllib.error\n")
	script.WriteString("from datetime import datetime\n")
	script.WriteString("\n")
	script.WriteString(fmt.Sprintf("endpoint = %q\n", cfg.S3Endpoint))
	script.WriteString(fmt.Sprintf("region = %q\n", cfg.S3Region))
	script.WriteString(fmt.Sprintf("bucket = %q\n", cfg.S3Bucket))
	script.WriteString(fmt.Sprintf("access_key = %q\n", cfg.S3AccessKey))
	script.WriteString(fmt.Sprintf("secret_key = base64.b64decode(%q).decode()\n", encodedSecret))
	script.WriteString(fmt.Sprintf("key = %q\n", mapKey))
	script.WriteString(fmt.Sprintf("local_path = %q\n", localPath))
	script.WriteString("if not region:\n    region = 'us-east-1'\n")
	script.WriteString("parsed = urllib.parse.urlparse(endpoint)\n")
	script.WriteString("if parsed.scheme not in ('http', 'https') or not parsed.netloc:\n    raise SystemExit('invalid s3 endpoint')\n")
	script.WriteString("path_prefix = parsed.path.rstrip('/')\n")
	script.WriteString("object_path = path_prefix + '/' + bucket + '/' + urllib.parse.quote(key, safe='/')\n")
	script.WriteString("url = f\"{parsed.scheme}://{parsed.netloc}{object_path}\"\n")
	script.WriteString("host = parsed.netloc\n")
	script.WriteString("with open(local_path, 'rb') as f:\n    body = f.read()\n")
	script.WriteString("def sign(key, msg):\n    return hmac.new(key, msg.encode(), hashlib.sha256).digest()\n")
	script.WriteString("def request(method, data):\n")
	script.WriteString("    payload_hash = hashlib.sha256(data if data is not None else b'').hexdigest()\n")
	script.WriteString("    amz_date = datetime.utcnow().strftime('%Y%m%dT%H%M%SZ')\n")
	script.WriteString("    date_stamp = amz_date[:8]\n")
	script.WriteString("    canonical_uri = object_path\n")
	script.WriteString("    canonical_headers = f'host:{host}\\n' + f'x-amz-content-sha256:{payload_hash}\\n' + f'x-amz-date:{amz_date}\\n'\n")
	script.WriteString("    signed_headers = 'host;x-amz-content-sha256;x-amz-date'\n")
	script.WriteString("    canonical_request = '\\n'.join([method, canonical_uri, '', canonical_headers, signed_headers, payload_hash])\n")
	script.WriteString("    credential_scope = f'{date_stamp}/{region}/s3/aws4_request'\n")
	script.WriteString("    string_to_sign = '\\n'.join(['AWS4-HMAC-SHA256', amz_date, credential_scope, hashlib.sha256(canonical_request.encode()).hexdigest()])\n")
	script.WriteString("    k_date = sign(('AWS4' + secret_key).encode(), date_stamp)\n")
	script.WriteString("    k_region = sign(k_date, region)\n")
	script.WriteString("    k_service = sign(k_region, 's3')\n")
	script.WriteString("    k_signing = sign(k_service, 'aws4_request')\n")
	script.WriteString("    signature = hmac.new(k_signing, string_to_sign.encode(), hashlib.sha256).hexdigest()\n")
	script.WriteString("    authorization = (\n")
	script.WriteString("        'AWS4-HMAC-SHA256 '\n")
	script.WriteString("        f'Credential={access_key}/{credential_scope}, '\n")
	script.WriteString("        f'SignedHeaders={signed_headers}, '\n")
	script.WriteString("        f'Signature={signature}'\n")
	script.WriteString("    )\n")
	script.WriteString("    req = urllib.request.Request(url, data=data, method=method)\n")
	script.WriteString("    req.add_header('Authorization', authorization)\n")
	script.WriteString("    req.add_header('x-amz-content-sha256', payload_hash)\n")
	script.WriteString("    req.add_header('x-amz-date', amz_date)\n")
	script.WriteString("    if data is not None:\n")
	script.WriteString("        req.add_header('Content-Length', str(len(data)))\n")
	script.WriteString("        req.add_header('Content-Type', 'application/octet-stream')\n")
	script.WriteString("    return req\n")
	script.WriteString("try:\n")
	script.WriteString("    with urllib.request.urlopen(request('HEAD', None)) as resp:\n")
	script.WriteString("        if 200 <= resp.getcode() < 300:\n")
	script.WriteString("            raise SystemExit('object already exists')\n")
	script.WriteString("except urllib.error.HTTPError as exc:\n")
	script.WriteString("    if exc.code not in (404, 403):\n")
	script.WriteString("        body = exc.read().decode(errors='ignore')\n")
	script.WriteString("        raise SystemExit(f'head failed: {exc.code} {body}')\n")
	script.WriteString("with urllib.request.urlopen(request('PUT', body)) as resp:\n")
	script.WriteString("    if resp.getcode() < 200 or resp.getcode() >= 300:\n")
	script.WriteString("        raise SystemExit(f'upload failed: {resp.getcode()}')\n")
	script.WriteString("with urllib.request.urlopen(request('HEAD', None)) as resp:\n")
	script.WriteString("    if resp.getcode() < 200 or resp.getcode() >= 300:\n")
	script.WriteString("        raise SystemExit(f'verify failed: {resp.getcode()}')\n")
	script.WriteString("print('uploaded')\n")
	return script.String()
}

func postWebRCONLevelURL(ctx context.Context, cfg wipeRuntimeConfig, mapURL string, logger *wipeLogger) error {
	base := strings.TrimRight(cfg.WebRCONAPIURL, "/")
	u := fmt.Sprintf("%s/api/servers/%d/command", base, cfg.WebRCONServerID)
	body := map[string]string{
		"command":  fmt.Sprintf("server.levelurl %q", mapURL),
		"password": cfg.DecryptedRCONPass,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	logger.Printf("已尝试下发 server.levelurl")
	return nil
}

func writeWipeState(path string, state wipeStateFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readLastWipeState(path string) (wipeStateFile, error) {
	var state wipeStateFile
	b, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return state, err
	}
	return state, nil
}

func buildMapURL(publicBaseURL string, key string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if !isPublicHTTPURL(base) {
		return "", errors.New("PUBLIC_BASE_URL 不是公网地址")
	}
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return base + "/" + strings.Join(parts, "/"), nil
}

func isPublicHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return true
}

func runHostShellCommand(ctx context.Context, shellScript string) (stdout string, stderr string, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, "sh", "-lc", shellScript)
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

func scanMapDir(mapDir string) (map[string]detectedMap, error) {
	entries, err := os.ReadDir(mapDir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]detectedMap, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".map") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out[name] = detectedMap{
			Filename: name,
			Path:     filepath.Join(mapDir, name),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
		}
	}
	return out, nil
}

func updateCommonConfig(path string, values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var existing []byte
	if b, err := os.ReadFile(path); err == nil {
		existing = b
	} else if !os.IsNotExist(err) {
		return err
	}

	lines := []string{}
	if len(existing) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(existing), "\r\n", "\n"), "\n")
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		replaced := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, key+"=") {
				lines[i] = fmt.Sprintf(`%s="%s"`, key, escapeConfigValue(values[key]))
				replaced = true
			}
		}
		if !replaced {
			lines = append(lines, fmt.Sprintf(`%s="%s"`, key, escapeConfigValue(values[key])))
		}
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func escapeConfigValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(value)
}

func pathJoinURL(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}
