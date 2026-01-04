package main

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

var errUserExists = errors.New("user exists")

var usernameFileRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

type storedUser struct {
	ID           uint64    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type storedServer struct {
	ID                 uint64    `json:"id"`
	UserID             uint64    `json:"user_id"`
	Name               string    `json:"name"`
	Alias              *string   `json:"alias,omitempty"`
	Summary            *string   `json:"summary,omitempty"`
	Host               string    `json:"host"`
	Port               uint16    `json:"port"`
	RconPasswordCipher string    `json:"rcon_password_ciphertext"`
	ContainerID        *string   `json:"container_id,omitempty"`
	ContainerName      *string   `json:"container_name,omitempty"`
	FilesPath          *string   `json:"files_path,omitempty"`
	LastWipeAt         string    `json:"last_wipe_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type storeMeta struct {
	Version      int    `json:"version"`
	NextUserID   uint64 `json:"next_user_id"`
	NextServerID uint64 `json:"next_server_id"`
}

type jsonStore struct {
	mu        sync.RWMutex
	baseDir   string
	usersDir  string
	serversDir string
	metaPath  string

	meta       storeMeta
	usersByID  map[uint64]storedUser
	usersByKey map[string]storedUser
	serversByID map[uint64]storedServer
}

func openJSONStore(baseDir string) (*jsonStore, error) {
	base := filepath.Clean(baseDir)
	usersDir := filepath.Join(base, "users")
	serversDir := filepath.Join(base, "servers")
	metaPath := filepath.Join(base, "meta.json")

	if err := os.MkdirAll(usersDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(serversDir, 0o755); err != nil {
		return nil, err
	}

	s := &jsonStore{
		baseDir:    base,
		usersDir:   usersDir,
		serversDir: serversDir,
		metaPath:   metaPath,
		meta: storeMeta{
			Version:      1,
			NextUserID:   1,
			NextServerID: 1,
		},
		usersByID:   make(map[uint64]storedUser),
		usersByKey:  make(map[string]storedUser),
		serversByID: make(map[uint64]storedServer),
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
	s.repairLocked()
	if err := s.migrateServersLocked(); err != nil {
		return nil, err
	}
	if err := s.persistMetaLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *jsonStore) migrateServersLocked() error {
	for id, srv := range s.serversByID {
		p := s.serverPath(id)
		b, err := os.ReadFile(p)
		if err == nil && bytes.Contains(b, []byte(`"last_wipe_at"`)) {
			continue
		}
		if err := writeJSONAtomic(p, srv); err != nil {
			return err
		}
	}
	return nil
}

func (s *jsonStore) loadMetaLocked() error {
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

func (s *jsonStore) persistMetaLocked() error {
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

func (s *jsonStore) loadUsersLocked() error {
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
		var u storedUser
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

func (s *jsonStore) loadServersLocked() error {
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
		var srv storedServer
		if err := json.Unmarshal(b, &srv); err != nil {
			return err
		}
		if srv.ID == 0 || srv.UserID == 0 {
			continue
		}
		s.serversByID[srv.ID] = srv
	}
	return nil
}

func (s *jsonStore) repairLocked() {
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

func (s *jsonStore) userPath(username string) string {
	return filepath.Join(s.usersDir, username+".json")
}

func (s *jsonStore) serverPath(id uint64) string {
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

func (s *jsonStore) CreateUser(username, passwordHash string) (storedUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !usernameFileRe.MatchString(username) {
		return storedUser{}, errors.New("invalid username")
	}
	if _, ok := s.usersByKey[username]; ok {
		return storedUser{}, errUserExists
	}
	u := storedUser{
		ID:           s.meta.NextUserID,
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	s.meta.NextUserID++
	if err := writeJSONAtomic(s.userPath(username), u); err != nil {
		return storedUser{}, err
	}
	s.usersByID[u.ID] = u
	s.usersByKey[username] = u
	if err := s.persistMetaLocked(); err != nil {
		return storedUser{}, err
	}
	return u, nil
}

func (s *jsonStore) FindUserByUsername(username string) (*storedUser, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.usersByKey[username]
	if !ok {
		return nil, false
	}
	out := u
	return &out, true
}

func (s *jsonStore) FindUserByID(id uint64) (*storedUser, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.usersByID[id]
	if !ok {
		return nil, false
	}
	out := u
	return &out, true
}

func (s *jsonStore) UpdateUserPasswordHash(username, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.usersByKey[username]
	if !ok {
		return os.ErrNotExist
	}
	u.PasswordHash = passwordHash
	if err := writeJSONAtomic(s.userPath(username), u); err != nil {
		return err
	}
	s.usersByKey[username] = u
	s.usersByID[u.ID] = u
	return nil
}

func (s *jsonStore) ListServersByUserID(userID uint64) []storedServer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]storedServer, 0)
	for _, srv := range s.serversByID {
		if srv.UserID == userID {
			out = append(out, srv)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *jsonStore) CreateServer(userID uint64, name, host string, port uint16, rconPasswordCipher string, containerName *string) (storedServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	srv := storedServer{
		ID:                 s.meta.NextServerID,
		UserID:             userID,
		Name:               name,
		Host:               host,
		Port:               port,
		RconPasswordCipher: rconPasswordCipher,
		ContainerName:      containerName,
		LastWipeAt:         "",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.meta.NextServerID++
	if err := writeJSONAtomic(s.serverPath(srv.ID), srv); err != nil {
		return storedServer{}, err
	}
	s.serversByID[srv.ID] = srv
	if err := s.persistMetaLocked(); err != nil {
		return storedServer{}, err
	}
	return srv, nil
}

func (s *jsonStore) FindServerByID(userID, id uint64) (*storedServer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.serversByID[id]
	if !ok || srv.UserID != userID {
		return nil, false
	}
	out := srv
	return &out, true
}

func (s *jsonStore) FindServerByIDAny(id uint64) (*storedServer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.serversByID[id]
	if !ok {
		return nil, false
	}
	out := srv
	return &out, true
}

func (s *jsonStore) UpdateServer(userID, id uint64, apply func(*storedServer) error) (storedServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.serversByID[id]
	if !ok || cur.UserID != userID {
		return storedServer{}, os.ErrNotExist
	}

	next := cur
	if err := apply(&next); err != nil {
		return storedServer{}, err
	}
	next.UpdatedAt = time.Now()
	if err := writeJSONAtomic(s.serverPath(next.ID), next); err != nil {
		return storedServer{}, err
	}
	s.serversByID[id] = next
	return next, nil
}

func (s *jsonStore) DeleteServer(userID, id uint64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	srv, ok := s.serversByID[id]
	if !ok || srv.UserID != userID {
		return false, nil
	}
	if err := os.Remove(s.serverPath(id)); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	delete(s.serversByID, id)
	return true, nil
}
