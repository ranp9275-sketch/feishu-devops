package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devops/feishu/config"
	"devops/feishu/pkg/feishu"
	_ "devops/feishu/pkg/handler"
	_ "devops/feishu/pkg/reg"
	_ "devops/feishu/pkg/robot/api"
	_ "devops/feishu/pkg/robot/impl"
	_ "devops/jenkins/oa-jenkins"

	_ "devops/oa/pkg/handler"
	"devops/tools/ioc"
	"devops/tools/logger"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 创建日志记录器
	log := logger.NewLogger(cfg.LogLevel)
	log.Info("Starting feishu message service...")

	// 启动回调监听（长连接）
	go func() {
		log.Info("Starting Feishu WebSocket client...")
		feishu.RegisterCallback(cfg)
	}()

	// 初始化 IOC 容器
	if err := ioc.ConController.Init(); err != nil {
		log.Fatal("Failed to init ioc: %v", err)
	}

	if err := ioc.Api.Init(); err != nil {
		log.Fatal("Failed to init ioc: %v", err)
	}

	// 注册 Prometheus 指标接口
	cfg.Application.GinServer().GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 配置HTTP服务器
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      cfg.Application.GinServer(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// 启动服务器
	go func() {
		log.Info("Server starting on port %s...", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// 设置可配置的超时时间来关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited")
}
