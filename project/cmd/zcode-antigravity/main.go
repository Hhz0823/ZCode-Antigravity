package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

const version = "0.6.2-test"

var defaultCommand = "menu"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	rootOverride := os.Getenv("ZCODE_ANTIGRAVITY_HOME")
	autoSetup := false
	global := flag.NewFlagSet("zcode-antigravity", flag.ContinueOnError)
	global.SetOutput(os.Stderr)
	global.StringVar(&rootOverride, "root", rootOverride, "override package root (for testing)")
	global.BoolVar(&autoSetup, "auto-setup", false, "start setup from the graphical control center")
	if err := global.Parse(args); err != nil {
		return 2
	}

	rest := global.Args()
	command := defaultCommand
	if len(rest) > 0 {
		command = strings.ToLower(strings.TrimSpace(rest[0]))
	}
	if command == "version" || command == "--version" || command == "-version" {
		fmt.Printf("ZCode Antigravity Bridge %s\n", version)
		return 0
	}
	if command == "help" || command == "--help" || command == "-h" {
		printHelp()
		return 0
	}

	app, err := newApp(rootOverride)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化失败: %v\n", err)
		return 1
	}
	if command == "gui" || command == "dashboard" {
		if errGUI := app.runGUI(autoSetup); errGUI != nil {
			fmt.Fprintf(os.Stderr, "图形控制中心失败: %v\n", errGUI)
			return 1
		}
		return 0
	}
	if command == "native-host" {
		if errHost := app.runNativeHost(autoSetup); errHost != nil {
			fmt.Fprintf(os.Stderr, "原生客户端后台失败: %v\n", errHost)
			return 1
		}
		return 0
	}
	releaseLock, errLock := app.acquireRunLock()
	if errLock != nil {
		fmt.Fprintf(os.Stderr, "初始化失败: %v\n", errLock)
		return 1
	}
	defer releaseLock()

	var commandErr error
	switch command {
	case "menu":
		commandErr = app.menu()
	case "setup":
		commandErr = app.setup()
	case "login":
		commandErr = app.login()
	case "login-grok", "login-xai":
		commandErr = app.loginGrok()
	case "start":
		commandErr = app.startAndConfigure()
	case "sync", "configure-zcode":
		commandErr = app.syncZCode()
	case "status":
		commandErr = app.status()
	case "quota":
		commandErr = app.printQuota()
	case "doctor", "self-test":
		commandErr = app.doctor()
	case "smoke", "test-model":
		model := "gemini-3.7-flash"
		if len(rest) > 1 && strings.TrimSpace(rest[1]) != "" {
			model = strings.TrimSpace(rest[1])
		}
		commandErr = app.smokeModel(model)
	case "stop":
		commandErr = app.stop()
	case "remove-provider", "uninstall-zcode":
		commandErr = app.removeZCodeProvider()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", command)
		printHelp()
		return 2
	}

	if commandErr != nil {
		fmt.Fprintf(os.Stderr, "失败: %v\n", commandErr)
		return 1
	}
	return 0
}

func printHelp() {
	fmt.Println(`ZCode Antigravity Bridge

用法:
  ZCode-Antigravity setup            首次登录、启动并配置 ZCode
  ZCode-Antigravity login            登录/添加 Antigravity 账号
  ZCode-Antigravity login-grok       登录/添加 Grok / xAI 账号
  ZCode-Antigravity start            启动网关并同步 ZCode Provider
  ZCode-Antigravity sync             重新同步模型列表到 ZCode
  ZCode-Antigravity status           查看端口、账号和模型状态
  ZCode-Antigravity quota            读取 Antigravity 模型额度
  ZCode-Antigravity gui              打开图形控制中心
  ZCode-Antigravity native-host      为 Swift/Rust 原生客户端启动本地 API
  ZCode-Antigravity doctor           运行本机检查
  ZCode-Antigravity smoke [model]    发送一次真实的小型模型请求
  ZCode-Antigravity stop             停止本程序启动的网关
  ZCode-Antigravity remove-provider  只从 ZCode 删除本 Provider

无参数运行会显示交互菜单。`)
}

func (a *app) menu() error {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\n========================================")
		fmt.Println(" ZCode Antigravity Bridge " + version)
		fmt.Println("========================================")
		fmt.Println("1. 首次设置（登录 + 启动 + 配置 ZCode）")
		fmt.Println("2. 启动网关并同步 ZCode")
		fmt.Println("3. 登录/添加 Antigravity 账号")
		fmt.Println("4. 查看状态")
		fmt.Println("5. 停止网关")
		fmt.Println("6. 从 ZCode 删除本 Provider")
		fmt.Println("7. 本机检查")
		fmt.Println("8. 真实测试 Gemini 3.7 Flash")
		fmt.Println("9. 登录/添加 Grok / xAI 账号")
		fmt.Println("0. 退出")
		fmt.Print("请选择: ")
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, os.ErrClosed) {
			return err
		}
		switch strings.TrimSpace(line) {
		case "1":
			if err := a.setup(); err != nil {
				fmt.Printf("\n操作失败: %v\n", err)
			}
		case "2":
			if err := a.startAndConfigure(); err != nil {
				fmt.Printf("\n操作失败: %v\n", err)
			}
		case "3":
			if err := a.login(); err != nil {
				fmt.Printf("\n操作失败: %v\n", err)
			}
		case "4":
			if err := a.status(); err != nil {
				fmt.Printf("\n状态异常: %v\n", err)
			}
		case "5":
			if err := a.stop(); err != nil {
				fmt.Printf("\n停止失败: %v\n", err)
			}
		case "6":
			if err := a.removeZCodeProvider(); err != nil {
				fmt.Printf("\n删除失败: %v\n", err)
			}
		case "7":
			if err := a.doctor(); err != nil {
				fmt.Printf("\n检查发现问题: %v\n", err)
			}
		case "8":
			if err := a.smokeModel("gemini-3.7-flash"); err != nil {
				fmt.Printf("\n模型测试失败: %v\n", err)
			}
		case "9":
			if err := a.loginGrok(); err != nil {
				fmt.Printf("\nGrok 登录失败: %v\n", err)
			}
		case "0", "q", "quit", "exit":
			return nil
		default:
			fmt.Println("请输入 0-8。")
		}
	}
}
