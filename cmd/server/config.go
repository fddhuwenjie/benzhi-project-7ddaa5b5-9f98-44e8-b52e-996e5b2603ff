package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	address      string
	dataPath     string
	selftest     bool
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func parseConfig(args []string) (config, error) {
	defaults, err := addressFromPort(os.Getenv("PORT"))
	if err != nil {
		return config{}, err
	}
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	var cfg config
	fs.StringVar(&cfg.address, "addr", defaults, "HTTP 监听地址")
	fs.StringVar(&cfg.dataPath, "data", "data/migration-cases.json", "JSON 快照路径")
	fs.BoolVar(&cfg.selftest, "selftest", false, "运行 HTTP 冒烟流程后退出")
	fs.DurationVar(&cfg.readTimeout, "read-timeout", 10*time.Second, "HTTP 读取超时")
	fs.DurationVar(&cfg.writeTimeout, "write-timeout", 15*time.Second, "HTTP 写入超时")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, errors.New("存在无法识别的位置参数")
	}
	if err := validateAddress(cfg.address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.dataPath) == "" {
		return config{}, errors.New("数据路径不能为空")
	}
	return cfg, nil
}

func addressFromPort(portText string) (string, error) {
	if strings.TrimSpace(portText) == "" {
		return defaultAddress, nil
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return "", fmt.Errorf("PORT 必须是 1024 至 65535 的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 格式无效：%w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("-addr 只允许回环地址")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return errors.New("-addr 端口须在 1024 至 65535 之间")
	}
	return nil
}
