package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/Sora233/MiraiGo-Template/config"
	"github.com/alecthomas/kong"
	"github.com/cnxysoft/DDBOT-WSa"
	_ "github.com/cnxysoft/DDBOT-WSa/logging"
	"github.com/cnxysoft/DDBOT-WSa/lsp"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/acfun"
	"github.com/cnxysoft/DDBOT-WSa/lsp/bilibili"
	localdb "github.com/cnxysoft/DDBOT-WSa/lsp/buntdb"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/douyin"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/douyu"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/huya"
	"github.com/cnxysoft/DDBOT-WSa/lsp/permission"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/roblox"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/twitter"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/weibo"
	_ "github.com/cnxysoft/DDBOT-WSa/lsp/youtube"
	_ "github.com/cnxysoft/DDBOT-WSa/msg-marker"
	"github.com/cnxysoft/DDBOT-WSa/warn"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate-config" {
		fs := flag.NewFlagSet("migrate-config", flag.ExitOnError)
		in := fs.String("in", "application.yaml", "legacy config input path")
		out := fs.String("out", "application.v2.yaml", "v2 config output path")
		_ = fs.Parse(os.Args[2:])

		report, err := config.MigrateConfigFile(*in, *out)
		if err != nil {
			fmt.Printf("migrate-config failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(config.FormatMigrationReport(report))
		return
	}

	var cli struct {
		Play         bool  `optional:"" help:"run play() for development"`
		Debug        bool  `optional:"" help:"enable debug mode"`
		SetAdmin     int64 `optional:"" xor:"c" help:"grant admin role to a QQ number"`
		Version      bool  `optional:"" xor:"c" short:"v" help:"print version information"`
		SyncBilibili bool  `optional:"" xor:"c" help:"sync bilibili follows for configured account"`
	}
	kong.Parse(&cli)

	if cli.Version {
		fmt.Printf("Tags: %v\n", lsp.Tags)
		fmt.Printf("COMMIT_ID: %v\n", lsp.CommitId)
		fmt.Printf("BUILD_TIME: %v\n", lsp.BuildTime)
		os.Exit(0)
	}

	if err := localdb.InitBuntDB(""); err != nil {
		if err == localdb.ErrLockNotHold {
			warn.Warn("failed to lock lsp.db: another bot process may already be running")
		} else {
			warn.Warn("unable to initialize database lsp.db")
		}
		return
	}

	if runtime.GOOS == "windows" {
		if err := exitHook(func() {
			localdb.Close()
		}); err != nil {
			localdb.Close()
			warn.Warn("unable to initialize Windows exit hook")
			return
		}
	} else {
		defer localdb.Close()
	}

	if cli.SetAdmin != 0 {
		sm := permission.NewStateManager()
		err := sm.GrantRole(cli.SetAdmin, permission.Admin)
		if err != nil {
			fmt.Printf("grant admin failed: %v\n", err)
		}
		return
	}

	if cli.SyncBilibili {
		config.Init()
		c := bilibili.NewConcern(nil)
		c.StateManager.FreshIndex()
		bilibili.Init()
		c.SyncSub()
		return
	}

	fmt.Println("DDBOT Nexus community groups: 1077402983")
	fmt.Println("Forks: https://github.com/Hoshinonyaruko/DDBOT-ws")
	fmt.Println("This branch: https://github.com/Yar1991-Translation/DDBOT-Nexus")
	fmt.Println("Supported frameworks: LLOneBot, NapCat, Lagrange")
	fmt.Println("LLOneBot: https://llonebot.github.io/")
	fmt.Println("NapCat: https://napneko.github.io/")
	fmt.Println("Lagrange: https://lagrangedev.github.io/Lagrange.Doc/")

	if cli.Debug {
		lsp.Debug = true
		go http.ListenAndServe("localhost:6060", nil)
	}

	if cli.Play {
		play()
		return
	}

	DDBOT.SetUpLog()
	DDBOT.Run()
}
