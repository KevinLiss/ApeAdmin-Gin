package main

import (
	"flag"
	"log"

	"apeadmin-gin/internal/bootstrap"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	if err := bootstrap.Run(*configPath); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
