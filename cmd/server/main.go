package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/httpui"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	if err := run(os.Args[1:]); err != nil {
		log.Printf("服务退出：%v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return fmt.Errorf("配置错误：%w", err)
	}
	if cfg.selftest {
		tempDir, err := os.MkdirTemp("", "benzhi-tree-review-selftest-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)
		cfg.dataPath = filepath.Join(tempDir, "snapshot.json")
	}
	repo, err := store.Open(cfg.dataPath)
	if err != nil {
		return fmt.Errorf("恢复数据失败：%w", err)
	}
	evaluator := policy.NewEvaluator(time.Now)
	service := application.NewService(repo, evaluator, time.Now, nil)
	ui := httpui.New(service)
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("监听 %s 失败：%w", cfg.address, err)
	}
	server := &http.Server{Handler: ui.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: cfg.readTimeout, WriteTimeout: cfg.writeTimeout, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	log.Printf("古树迁移联合审查台已监听 http://%s", cfg.address)
	if cfg.selftest {
		return runSelftestAndShutdown(server, serveErr, cfg.address)
	}
	return waitAndShutdown(server, serveErr)
}

func waitAndShutdown(server *http.Server, serveErr <-chan error) error {
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-signalContext.Done():
		log.Printf("收到关闭信号，开始停止服务")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务关闭超时：%w", err)
	}
	err := <-serveErr
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Printf("服务已安全关闭")
	return nil
}

func runSelftestAndShutdown(server *http.Server, serveErr <-chan error, address string) error {
	testErr := executeSelftest("http://" + address)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(ctx)
	serverErr := <-serveErr
	if testErr != nil {
		return fmt.Errorf("自检失败：%w", testErr)
	}
	if shutdownErr != nil {
		return fmt.Errorf("自检后关闭失败：%w", shutdownErr)
	}
	if serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
		return serverErr
	}
	log.Printf("自检通过：完整流程已到达 approved")
	return nil
}
