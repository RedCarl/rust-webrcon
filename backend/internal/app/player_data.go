package app

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

type playerDataSchemaResponse struct {
	Tables []playerDataSchemaTable `json:"tables"`
}

type playerDataSchemaTable struct {
	Name    string                   `json:"name"`
	Columns []playerDataSchemaColumn `json:"columns"`
}

type playerDataSchemaColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsNullable bool   `json:"is_nullable"`
	Key        string `json:"key"`
}

type playerBanRow struct {
	ServerName string `json:"server_name"`
	PlayerName string `json:"player_name"`
	SteamID    string `json:"steam_id"`
	Reason     string `json:"reason"`
	CreatedAt  string `json:"created_at"`
	ExpiresAt  string `json:"expires_at"`
	BannedBy   string `json:"banned_by"`
	UnbannedAt string `json:"unbanned_at"`
}

type playerReportRow struct {
	ServerName      string `json:"server_name"`
	ReporterName    string `json:"reporter_name"`
	ReporterSteamID string `json:"reporter_steam_id"`
	TargetName      string `json:"target_name"`
	TargetSteamID   string `json:"target_steam_id"`
	Subject         string `json:"subject"`
	Reason          string `json:"reason"`
	Type            string `json:"type"`
	CreatedAt       string `json:"created_at"`
}

type playerStatsRow struct {
	PlayerName             string `json:"player_name"`
	SteamID                string `json:"steam_id"`
	ServerCount            int64  `json:"server_count"`
	WipeCount              int64  `json:"wipe_count"`
	TotalOnlineTimeSeconds int64  `json:"total_online_time_seconds"`
	LastSeenAt             string `json:"last_seen_at"`
}

func (a *App) ginAdminGetPlayerDataBans(c *gin.Context) {
	db, err := a.openPlayerDataDB()
	if err != nil {
		c.JSON(400, apiErrorBody{Message: err.Error()})
		return
	}
	defer db.Close()

	args := make([]any, 0, 2)
	query := `
		SELECT
			COALESCE(server_name, '') AS server_name,
			COALESCE(player_name, '') AS player_name,
			COALESCE(steam_id, '') AS steam_id,
			COALESCE(reason, '') AS reason,
			COALESCE(DATE_FORMAT(ban_time, '%Y-%m-%d %H:%i:%s'), '') AS created_at,
			COALESCE(DATE_FORMAT(expire_time, '%Y-%m-%d %H:%i:%s'), '') AS expires_at,
			COALESCE(banned_by_name, '') AS banned_by,
			COALESCE(DATE_FORMAT(unban_time, '%Y-%m-%d %H:%i:%s'), '') AS unbanned_at
		FROM player_bans
	`
	serverName := strings.TrimSpace(c.Query("server_name"))
	if serverName != "" {
		query += ` WHERE server_name = ?`
		args = append(args, serverName)
	}
	query += ` ORDER BY ban_time DESC LIMIT ?`
	args = append(args, clampLimit(c.Query("limit"), 100, 500))

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "读取玩家封禁列表失败"})
		return
	}
	defer rows.Close()

	result := make([]playerBanRow, 0, 64)
	for rows.Next() {
		var item playerBanRow
		if err := rows.Scan(
			&item.ServerName,
			&item.PlayerName,
			&item.SteamID,
			&item.Reason,
			&item.CreatedAt,
			&item.ExpiresAt,
			&item.BannedBy,
			&item.UnbannedAt,
		); err != nil {
			c.JSON(500, apiErrorBody{Message: "读取玩家封禁列表失败"})
			return
		}
		result = append(result, item)
	}

	c.JSON(200, gin.H{"rows": result})
}

func (a *App) ginAdminGetPlayerDataReports(c *gin.Context) {
	db, err := a.openPlayerDataDB()
	if err != nil {
		c.JSON(400, apiErrorBody{Message: err.Error()})
		return
	}
	defer db.Close()

	args := make([]any, 0, 2)
	query := `
		SELECT
			COALESCE(server_name, '') AS server_name,
			COALESCE(reporter_name, '') AS reporter_name,
			COALESCE(reporter_id, '') AS reporter_steam_id,
			COALESCE(suspect_name, '') AS target_name,
			COALESCE(suspect_id, '') AS target_steam_id,
			COALESCE(report_subject, '') AS subject,
			COALESCE(report_message, '') AS reason,
			COALESCE(report_type, '') AS type,
			COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), '') AS created_at
		FROM player_reports
	`
	serverName := strings.TrimSpace(c.Query("server_name"))
	if serverName != "" {
		query += ` WHERE server_name = ?`
		args = append(args, serverName)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, clampLimit(c.Query("limit"), 100, 500))

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "读取玩家举报列表失败"})
		return
	}
	defer rows.Close()

	result := make([]playerReportRow, 0, 64)
	for rows.Next() {
		var item playerReportRow
		if err := rows.Scan(
			&item.ServerName,
			&item.ReporterName,
			&item.ReporterSteamID,
			&item.TargetName,
			&item.TargetSteamID,
			&item.Subject,
			&item.Reason,
			&item.Type,
			&item.CreatedAt,
		); err != nil {
			c.JSON(500, apiErrorBody{Message: "读取玩家举报列表失败"})
			return
		}
		result = append(result, item)
	}

	c.JSON(200, gin.H{"rows": result})
}

func (a *App) ginAdminGetPlayerDataStats(c *gin.Context) {
	db, err := a.openPlayerDataDB()
	if err != nil {
		c.JSON(400, apiErrorBody{Message: err.Error()})
		return
	}
	defer db.Close()

	args := make([]any, 0, 8)
	conditions := make([]string, 0, 3)

	serverName := strings.TrimSpace(c.Query("server_name"))
	if serverName != "" {
		conditions = append(conditions, "server_name = ?")
		args = append(args, serverName)
	}
	playerName := strings.TrimSpace(c.Query("player_name"))
	if playerName != "" {
		conditions = append(conditions, "player_name LIKE ?")
		args = append(args, "%"+playerName+"%")
	}
	steamID := strings.TrimSpace(c.Query("steam_id"))
	if steamID != "" {
		conditions = append(conditions, "player_id LIKE ?")
		args = append(args, "%"+steamID+"%")
	}

	query := `
		SELECT
			COALESCE(player_name, '') AS player_name,
			COALESCE(player_id, '') AS steam_id,
			COUNT(DISTINCT server_name) AS server_count,
			COUNT(DISTINCT wipe_date) AS wipe_count,
			COALESCE(SUM(online_time_seconds), 0) AS total_online_time_seconds,
			COALESCE(DATE_FORMAT(MAX(updated_at), '%Y-%m-%d %H:%i:%s'), '') AS last_seen_at
		FROM stats_player_data
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += `
		GROUP BY player_id, player_name
		ORDER BY ` + normalizeStatsSortBy(c.Query("sort_by")) + ` ` + normalizeSortOrder(c.Query("sort_order")) + `
		LIMIT ?
	`
	args = append(args, clampLimit(c.Query("limit"), 100, 500))

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(500, apiErrorBody{Message: "读取玩家统计失败"})
		return
	}
	defer rows.Close()

	result := make([]playerStatsRow, 0, 64)
	for rows.Next() {
		var item playerStatsRow
		if err := rows.Scan(
			&item.PlayerName,
			&item.SteamID,
			&item.ServerCount,
			&item.WipeCount,
			&item.TotalOnlineTimeSeconds,
			&item.LastSeenAt,
		); err != nil {
			c.JSON(500, apiErrorBody{Message: "读取玩家统计失败"})
			return
		}
		result = append(result, item)
	}

	c.JSON(200, gin.H{"rows": result})
}

func (a *App) openPlayerDataDB() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		a.PlayerDataMySQLUser,
		a.PlayerDataMySQLPassword,
		a.PlayerDataMySQLHost,
		a.PlayerDataMySQLPort,
		a.PlayerDataMySQLDatabase,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	return db, nil
}

func clampLimit(raw string, def int, max int) int {
	text := strings.TrimSpace(raw)
	if text == "" {
		return def
	}
	value, err := strconv.Atoi(text)
	if err != nil || value <= 0 {
		return def
	}
	if value > max {
		return max
	}
	return value
}

func normalizeSortOrder(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "asc") {
		return "ASC"
	}
	return "DESC"
}

func normalizeStatsSortBy(raw string) string {
	switch strings.TrimSpace(raw) {
	case "wipe_count":
		return "wipe_count"
	case "server_count":
		return "server_count"
	case "player_name":
		return "player_name"
	case "last_seen_at":
		return "last_seen_at"
	default:
		return "total_online_time_seconds"
	}
}
