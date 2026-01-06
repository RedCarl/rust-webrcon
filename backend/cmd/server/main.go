package main

import (
	"flag"
	"log"

	"rust-panel-backend/internal/app"
	"rust-panel-backend/internal/config"
)

func main() {
	configPath := flag.String("config", "./config.json", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("读取配置文件失败：%v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置文件不合法：%v", err)
	}

	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("初始化失败：%v", err)
	}

	addr := cfg.BindAddr
	log.Printf("[%s] 后端启动：%s", a.AuthorLabel, addr)

	router := a.Router()
	if err := router.Run(addr); err != nil {
		log.Fatalf("服务异常退出：%v", err)
	}
}
