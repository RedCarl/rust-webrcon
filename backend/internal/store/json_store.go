package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrUserExists = errors.New("user exists")

var usernameFileRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

type User struct {
	ID        uint64    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
}

type Server struct {
	ID                     uint64      `json:"id"`
	UserIDs                []uint64    `json:"user_ids"`
	UserID                 uint64      `json:"user_id,omitempty"`
	Name                   string      `json:"name"`
	Alias                  *string     `json:"alias,omitempty"`
	Summary                *string     `json:"summary,omitempty"`
	Host                   string      `json:"host"`
	Port                   uint16      `json:"port"`
	RconPassword           string      `json:"rcon_password"`
	ContainerID            *string     `json:"container_id,omitempty"`
	ContainerName          *string     `json:"container_name,omitempty"`
	FilesPath              *string     `json:"files_path,omitempty"`
	WipeConfig             *WipeConfig `json:"wipe_config,omitempty"`
	LastWipeAt             string      `json:"last_wipe_at"`
	AlwaysUseTodayWipeDate bool        `json:"always_use_today_wipe_date,omitempty"`
	CreatedAt              time.Time   `json:"created_at"`
	UpdatedAt              time.Time   `json:"updated_at"`
}

type WipeConfig struct {
	LGSMDir              string  `json:"lgsm_dir,omitempty"`
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

type storeMeta struct {
	Version      int    `json:"version"`
	NextUserID   uint64 `json:"next_user_id"`
	NextServerID uint64 `json:"next_server_id"`
}

type JSONStore struct {
	mu           sync.RWMutex
	baseDir      string
	usersDir     string
	serversDir   string
	metaPath     string
	settingsPath string

	meta        storeMeta
	settings    GlobalSettings
	usersByID   map[uint64]User
	usersByKey  map[string]User
	serversByID map[uint64]Server
}

type GlobalSettings struct {
	WipeConfig       *WipeConfig       `json:"wipe_config,omitempty"`
	PlayerDataConfig *PlayerDataConfig `json:"player_data_config,omitempty"`
}

type PlayerDataConfig struct {
	MySQL       *MySQLConfig       `json:"mysql,omitempty"`
	BanList     *BanListConfig     `json:"ban_list,omitempty"`
	ReportList  *ReportListConfig  `json:"report_list,omitempty"`
	PlayerStats *PlayerStatsConfig `json:"player_stats,omitempty"`
}

type MySQLConfig struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
}

func (u *User) UnmarshalJSON(data []byte) error {
	type alias User
	var raw struct {
		alias
		LegacyPassword string `json:"password_hash"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = User(raw.alias)
	if u.Password == "" {
		u.Password = raw.LegacyPassword
	}
	return nil
}

func (s *Server) UnmarshalJSON(data []byte) error {
	type alias Server
	var raw struct {
		alias
		LegacyRconPassword string `json:"rcon_password_ciphertext"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = Server(raw.alias)
	if s.RconPassword == "" {
		s.RconPassword = raw.LegacyRconPassword
	}
	return nil
}

func (c *WipeConfig) UnmarshalJSON(data []byte) error {
	type alias WipeConfig
	var raw struct {
		alias
		LegacyS3SecretKey string `json:"s3_secret_key_ciphertext"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = WipeConfig(raw.alias)
	if c.S3SecretKey == "" {
		c.S3SecretKey = raw.LegacyS3SecretKey
	}
	return nil
}

func (c *MySQLConfig) UnmarshalJSON(data []byte) error {
	type alias MySQLConfig
	var raw struct {
		alias
		LegacyPassword string `json:"password_ciphertext"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = MySQLConfig(raw.alias)
	if c.Password == "" {
		c.Password = raw.LegacyPassword
	}
	return nil
}

type BanListConfig struct {
	TableName        string `json:"table_name,omitempty"`
	ServerNameColumn string `json:"server_name_column,omitempty"`
	PlayerNameColumn string `json:"player_name_column,omitempty"`
	SteamIDColumn    string `json:"steam_id_column,omitempty"`
	ReasonColumn     string `json:"reason_column,omitempty"`
	CreatedAtColumn  string `json:"created_at_column,omitempty"`
	ExpiresAtColumn  string `json:"expires_at_column,omitempty"`
	BannedByColumn   string `json:"banned_by_column,omitempty"`
}

type ReportListConfig struct {
	TableName             string `json:"table_name,omitempty"`
	ServerNameColumn      string `json:"server_name_column,omitempty"`
	ReporterNameColumn    string `json:"reporter_name_column,omitempty"`
	ReporterSteamIDColumn string `json:"reporter_steam_id_column,omitempty"`
	TargetNameColumn      string `json:"target_name_column,omitempty"`
	TargetSteamIDColumn   string `json:"target_steam_id_column,omitempty"`
	ReasonColumn          string `json:"reason_column,omitempty"`
	StatusColumn          string `json:"status_column,omitempty"`
	CreatedAtColumn       string `json:"created_at_column,omitempty"`
}

type PlayerStatsConfig struct {
	TableName        string `json:"table_name,omitempty"`
	ServerNameColumn string `json:"server_name_column,omitempty"`
	PlayerNameColumn string `json:"player_name_column,omitempty"`
	SteamIDColumn    string `json:"steam_id_column,omitempty"`
	WipeDateColumn   string `json:"wipe_date_column,omitempty"`
	PlayTimeColumn   string `json:"play_time_column,omitempty"`
	PlayTimeUnit     string `json:"play_time_unit,omitempty"`
	LastSeenAtColumn string `json:"last_seen_at_column,omitempty"`
}

func OpenJSONStore(baseDir string) (*JSONStore, error) {
	base := filepath.Clean(baseDir)
	usersDir := filepath.Join(base, "users")
	serversDir := filepath.Join(base, "servers")
	metaPath := filepath.Join(base, "meta.json")
	settingsPath := filepath.Join(base, "settings.json")

	if err := os.MkdirAll(usersDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(serversDir, 0o755); err != nil {
		return nil, err
	}

	s := &JSONStore{
		baseDir:      base,
		usersDir:     usersDir,
		serversDir:   serversDir,
		metaPath:     metaPath,
		settingsPath: settingsPath,
		meta: storeMeta{
			Version:      1,
			NextUserID:   1,
			NextServerID: 1,
		},
		usersByID:   make(map[uint64]User),
		usersByKey:  make(map[string]User),
		serversByID: make(map[uint64]Server),
	}

	if err := s.loadMetaLocked(); err != nil {
		return nil, err
	}
	if err := s.loadUsersLocked(); err != nil {
		return nil, err
	}
	if err := s.loadServersLocked(); err != nil {
		return nil, err
	}
	if err := s.loadSettingsLocked(); err != nil {
		return nil, err
	}
	s.repairLocked()
	if err := s.migrateUsersLocked(); err != nil {
		return nil, err
	}
	if err := s.migrateServersLocked(); err != nil {
		return nil, err
	}
	if err := s.migrateSettingsLocked(); err != nil {
		return nil, err
	}
	if err := s.persistMetaLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONStore) migrateUsersLocked() error {
	for _, u := range s.usersByID {
		p := s.userPath(u.Username)
		b, err := os.ReadFile(p)
		if err == nil && !containsLegacyPasswordFields(b) {
			continue
		}
		if err := writeJSONAtomic(p, u); err != nil {
			return err
		}
	}
	return nil
}

func (s *JSONStore) migrateServersLocked() error {
	for id, srv := range s.serversByID {
		p := s.serverPath(id)
		b, err := os.ReadFile(p)
		if err == nil && bytes.Contains(b, []byte(`"last_wipe_at"`)) && bytes.Contains(b, []byte(`"user_ids"`)) && !containsLegacyPasswordFields(b) {
			continue
		}
		if err := writeJSONAtomic(p, srv); err != nil {
			return err
		}
	}
	return nil
}

func (s *JSONStore) migrateSettingsLocked() error {
	b, err := os.ReadFile(s.settingsPath)
	if err == nil && !containsLegacyPasswordFields(b) {
		return nil
	}
	return s.persistSettingsLocked()
}

func containsLegacyPasswordFields(b []byte) bool {
	return bytes.Contains(b, []byte(`"password_hash"`)) ||
		bytes.Contains(b, []byte(`"password_ciphertext"`)) ||
		bytes.Contains(b, []byte(`"rcon_password_ciphertext"`)) ||
		bytes.Contains(b, []byte(`"s3_secret_key_ciphertext"`))
}

func (s *JSONStore) loadSettingsLocked() error {
	b, err := os.ReadFile(s.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.persistSettingsLocked()
		}
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return s.persistSettingsLocked()
	}
	return json.Unmarshal(b, &s.settings)
}

func (s *JSONStore) persistSettingsLocked() error {
	return writeJSONAtomic(s.settingsPath, s.settings)
}

func (s *JSONStore) loadMetaLocked() error {
	b, err := os.ReadFile(s.metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.persistMetaLocked()
		}
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return s.persistMetaLocked()
	}
	if err := json.Unmarshal(b, &s.meta); err != nil {
		return err
	}
	if s.meta.Version == 0 {
		s.meta.Version = 1
	}
	if s.meta.NextUserID == 0 {
		s.meta.NextUserID = 1
	}
	if s.meta.NextServerID == 0 {
		s.meta.NextServerID = 1
	}
	return nil
}

func (s *JSONStore) persistMetaLocked() error {
	b, err := json.MarshalIndent(s.meta, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.metaPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.metaPath)
}

func (s *JSONStore) loadUsersLocked() error {
	entries, err := os.ReadDir(s.usersDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p := filepath.Join(s.usersDir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		var u User
		if err := json.Unmarshal(b, &u); err != nil {
			return err
		}
		if u.ID == 0 || strings.TrimSpace(u.Username) == "" {
			continue
		}
		key := strings.TrimSpace(u.Username)
		s.usersByID[u.ID] = u
		s.usersByKey[key] = u
	}
	return nil
}

func (s *JSONStore) loadServersLocked() error {
	entries, err := os.ReadDir(s.serversDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p := filepath.Join(s.serversDir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		var srv Server
		if err := json.Unmarshal(b, &srv); err != nil {
			return err
		}
		if srv.ID == 0 {
			continue
		}
		if len(srv.UserIDs) == 0 {
			if srv.UserID != 0 {
				srv.UserIDs = []uint64{srv.UserID}
			} else {
				continue
			}
		}
		srv.UserIDs = CleanUserIDs(srv.UserIDs)
		if len(srv.UserIDs) == 0 {
			continue
		}
		srv.UserID = 0
		s.serversByID[srv.ID] = srv
	}
	return nil
}

func CleanUserIDs(ids []uint64) []uint64 {
	if len(ids) == 0 {
		return nil
	}
	m := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := m[id]; ok {
			continue
		}
		m[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	if len(out) == 0 {
		return nil
	}
	return out
}

func serverHasUser(srv *Server, userID uint64) bool {
	for _, id := range srv.UserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func (s *JSONStore) repairLocked() {
	var maxUserID uint64
	for id := range s.usersByID {
		if id > maxUserID {
			maxUserID = id
		}
	}
	if s.meta.NextUserID <= maxUserID {
		s.meta.NextUserID = maxUserID + 1
	}

	var maxServerID uint64
	for id := range s.serversByID {
		if id > maxServerID {
			maxServerID = id
		}
	}
	if s.meta.NextServerID <= maxServerID {
		s.meta.NextServerID = maxServerID + 1
	}
}

func (s *JSONStore) userPath(username string) string {
	return filepath.Join(s.usersDir, username+".json")
}

func (s *JSONStore) serverPath(id uint64) string {
	return filepath.Join(s.serversDir, fmt.Sprintf("%d.json", id))
}

func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *JSONStore) CreateUser(username, password string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !usernameFileRe.MatchString(username) {
		return User{}, errors.New("invalid username")
	}
	if _, ok := s.usersByKey[username]; ok {
		return User{}, ErrUserExists
	}
	u := User{
		ID:        s.meta.NextUserID,
		Username:  username,
		Password:  password,
		CreatedAt: time.Now(),
	}
	s.meta.NextUserID++
	if err := writeJSONAtomic(s.userPath(username), u); err != nil {
		return User{}, err
	}
	s.usersByID[u.ID] = u
	s.usersByKey[username] = u
	if err := s.persistMetaLocked(); err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *JSONStore) FindUserByUsername(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.usersByKey[username]
	if !ok {
		return nil, false
	}
	out := u
	return &out, true
}

func (s *JSONStore) FindUserByID(id uint64) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.usersByID[id]
	if !ok {
		return nil, false
	}
	out := u
	return &out, true
}

func (s *JSONStore) UpdateUserPassword(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.usersByKey[username]
	if !ok {
		return os.ErrNotExist
	}
	u.Password = password
	if err := writeJSONAtomic(s.userPath(username), u); err != nil {
		return err
	}
	s.usersByKey[username] = u
	s.usersByID[u.ID] = u
	return nil
}

func (s *JSONStore) ListServersByUserID(userID uint64) []Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Server, 0)
	for _, srv := range s.serversByID {
		if serverHasUser(&srv, userID) {
			out = append(out, srv)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *JSONStore) CreateServer(userIDs []uint64, name, host string, port uint16, rconPassword string, containerName *string, wipeConfig *WipeConfig) (Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userIDs = CleanUserIDs(userIDs)
	if len(userIDs) == 0 {
		return Server{}, errors.New("missing user ids")
	}

	now := time.Now()
	srv := Server{
		ID:            s.meta.NextServerID,
		UserID:        0,
		UserIDs:       userIDs,
		Name:          name,
		Host:          host,
		Port:          port,
		RconPassword:  rconPassword,
		ContainerName: containerName,
		WipeConfig:    wipeConfig,
		LastWipeAt:    "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.meta.NextServerID++
	if err := writeJSONAtomic(s.serverPath(srv.ID), srv); err != nil {
		return Server{}, err
	}
	s.serversByID[srv.ID] = srv
	if err := s.persistMetaLocked(); err != nil {
		return Server{}, err
	}
	return srv, nil
}

func (s *JSONStore) FindServerByID(userID, id uint64) (*Server, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.serversByID[id]
	if !ok || !serverHasUser(&srv, userID) {
		return nil, false
	}
	out := srv
	return &out, true
}

func (s *JSONStore) FindServerByIDAny(id uint64) (*Server, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.serversByID[id]
	if !ok {
		return nil, false
	}
	out := srv
	return &out, true
}

func (s *JSONStore) UpdateServer(userID, id uint64, apply func(*Server) error) (Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.serversByID[id]
	if !ok || !serverHasUser(&cur, userID) {
		return Server{}, os.ErrNotExist
	}

	next := cur
	if err := apply(&next); err != nil {
		return Server{}, err
	}
	next.UpdatedAt = time.Now()
	if err := writeJSONAtomic(s.serverPath(next.ID), next); err != nil {
		return Server{}, err
	}
	s.serversByID[id] = next
	return next, nil
}

func (s *JSONStore) DeleteServer(userID, id uint64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv, ok := s.serversByID[id]
	if !ok || !serverHasUser(&srv, userID) {
		return false, nil
	}
	if len(srv.UserIDs) > 1 {
		next := srv
		next.UserIDs = removeUserID(next.UserIDs, userID)
		next.UpdatedAt = time.Now()
		if err := writeJSONAtomic(s.serverPath(next.ID), next); err != nil {
			return false, err
		}
		s.serversByID[id] = next
		return true, nil
	}
	if err := os.Remove(s.serverPath(id)); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	delete(s.serversByID, id)
	return true, nil
}

func removeUserID(ids []uint64, userID uint64) []uint64 {
	if len(ids) == 0 {
		return nil
	}
	dst := ids[:0]
	for _, id := range ids {
		if id != userID {
			dst = append(dst, id)
		}
	}
	return CleanUserIDs(dst)
}

func (s *JSONStore) ListUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.usersByID))
	for _, u := range s.usersByID {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *JSONStore) ListServersAny() []Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Server, 0, len(s.serversByID))
	for _, srv := range s.serversByID {
		out = append(out, srv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *JSONStore) UpdateServerAny(id uint64, apply func(*Server) error) (Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.serversByID[id]
	if !ok {
		return Server{}, os.ErrNotExist
	}

	next := cur
	if err := apply(&next); err != nil {
		return Server{}, err
	}
	next.UpdatedAt = time.Now()
	if err := writeJSONAtomic(s.serverPath(next.ID), next); err != nil {
		return Server{}, err
	}
	s.serversByID[id] = next
	return next, nil
}

func (s *JSONStore) DeleteServerAny(id uint64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.serversByID[id]; !ok {
		return false, nil
	}
	if err := os.Remove(s.serverPath(id)); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	delete(s.serversByID, id)
	return true, nil
}

func (s *JSONStore) GetGlobalSettings() GlobalSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneGlobalSettings(s.settings)
}

func (s *JSONStore) UpdateGlobalSettings(apply func(*GlobalSettings) error) (GlobalSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneGlobalSettings(s.settings)
	if err := apply(&next); err != nil {
		return GlobalSettings{}, err
	}
	if err := writeJSONAtomic(s.settingsPath, next); err != nil {
		return GlobalSettings{}, err
	}
	s.settings = next
	return next, nil
}

func cloneGlobalSettings(src GlobalSettings) GlobalSettings {
	out := src
	if src.WipeConfig != nil {
		cfg := *src.WipeConfig
		out.WipeConfig = &cfg
	}
	if src.PlayerDataConfig != nil {
		cfg := *src.PlayerDataConfig
		out.PlayerDataConfig = &cfg
		if src.PlayerDataConfig.MySQL != nil {
			mysqlCfg := *src.PlayerDataConfig.MySQL
			out.PlayerDataConfig.MySQL = &mysqlCfg
		}
		if src.PlayerDataConfig.BanList != nil {
			banCfg := *src.PlayerDataConfig.BanList
			out.PlayerDataConfig.BanList = &banCfg
		}
		if src.PlayerDataConfig.ReportList != nil {
			reportCfg := *src.PlayerDataConfig.ReportList
			out.PlayerDataConfig.ReportList = &reportCfg
		}
		if src.PlayerDataConfig.PlayerStats != nil {
			statsCfg := *src.PlayerDataConfig.PlayerStats
			out.PlayerDataConfig.PlayerStats = &statsCfg
		}
	}
	return out
}
