package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type rconPacket struct {
	Identifier int    `json:"Identifier"`
	Message    string `json:"Message"`
	Name       string `json:"Name"`
	Type       string `json:"Type,omitempty"`
}

func (a *App) sendRconCommand(host string, port uint16, password, command string) (string, error) {
	// 参考前端逻辑：
	// 1. 去除两端空格
	// 2. 检查是否显式包含 ws:// 或 wss://
	// 3. 如果包含协议头，且有密码，则直接拼接 /encode(password)
	// 4. 如果没有协议头，则默认补 ws://，并拼接 /encode(password)

	addr := strings.TrimSpace(host)
	pw := strings.TrimSpace(password)
	isExplicitWsUrl := strings.HasPrefix(strings.ToLower(addr), "ws://") || strings.HasPrefix(strings.ToLower(addr), "wss://")

	var socketUrl string
	if isExplicitWsUrl {
		// 如果是显式 URL
		normalizedAddr := strings.TrimRight(addr, "/")
		if pw != "" {
			socketUrl = fmt.Sprintf("%s/%s", normalizedAddr, url.PathEscape(pw))
		} else {
			socketUrl = normalizedAddr
		}
	} else {
		// 如果不是显式 URL，默认补 ws://
		// 并在 host 后追加端口（如果指定了端口）
		normalizedAddr := strings.TrimRight(addr, "/")
		if port > 0 {
			// 如果 host 中没有端口，才追加端口
			// 简单的判断方法：看最后是否有 :数字
			// 但考虑到 IPv6 等复杂情况，这里我们假设如果用户填了 port > 0，就应该用这个端口
			// 除非 host 已经包含端口。
			// 为了简化，我们遵循：如果 port > 0，则强制使用该端口（如果 host 里没写端口）
			// 或者更简单：如果 port > 0，我们尝试解析 host，看是否已有端口
			if !strings.Contains(normalizedAddr, ":") {
				normalizedAddr = fmt.Sprintf("%s:%d", normalizedAddr, port)
			}
		}
		socketUrl = fmt.Sprintf("ws://%s/%s", normalizedAddr, url.PathEscape(pw))
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	// 尝试连接
	c, resp, err := dialer.Dial(socketUrl, nil)
	if err != nil {
		// 检查是否需要重定向 (301 Moved Permanently)
		// 只有当原始协议是 ws 时才尝试升级到 wss
		if resp != nil && resp.StatusCode == 301 && strings.HasPrefix(socketUrl, "ws://") {
			// 升级到 wss
			upgradeUrl := strings.Replace(socketUrl, "ws://", "wss://", 1)
			c, resp, err = dialer.Dial(upgradeUrl, nil)
			if err != nil {
				if resp != nil {
					return "", fmt.Errorf("connect error (after upgrade): %w (status: %s)", err, resp.Status)
				}
				return "", fmt.Errorf("connect error (after upgrade): %w", err)
			}
		} else {
			if resp != nil {
				return "", fmt.Errorf("connect error: %w (status: %s)", err, resp.Status)
			}
			return "", fmt.Errorf("connect error: %w", err)
		}
	}
	defer c.Close()

	req := rconPacket{
		Identifier: 1001,
		Message:    command,
		Name:       "WebRcon",
	}

	if err := c.WriteJSON(req); err != nil {
		return "", fmt.Errorf("write error: %w", err)
	}

	// Set read deadline
	c.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read response loop
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("read error: %w", err)
		}

		var resp rconPacket
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}

		if resp.Identifier == 1001 {
			return resp.Message, nil
		}
	}
}
