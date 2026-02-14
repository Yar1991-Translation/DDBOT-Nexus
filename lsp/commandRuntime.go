package lsp

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
	"github.com/cnxysoft/DDBOT-WSa/lsp/parser"
	localutils "github.com/cnxysoft/DDBOT-WSa/utils"
)

type Runtime struct {
	bot *localutils.HackedBot
	l   *Lsp
	*parser.Parser

	debug   bool
	exit    bool
	silence bool

	groupCode int64
	senderUin int64
}

func (r *Runtime) Exit(int) {
	r.exit = true
}

func (r *Runtime) Debug() {
	r.debug = true
}

func (r *Runtime) parseCommandSyntax(ast interface{}, name string, options ...kong.Option) (*kong.Context, string) {
	args := r.GetArgs()
	cmdOut := &strings.Builder{}
	options = append(options, kong.Name(name), kong.UsageOnError(), kong.Exit(r.Exit))
	k, err := kong.New(ast, options...)
	if err != nil {
		logger.Errorf("kong new failed %v", err)
		r.Exit(0)
		return nil, ""
	}
	if r.silence {
		k.Stdout = io.Discard
	} else {
		k.Stdout = cmdOut
	}
	ctx, err := k.Parse(args)
	if r.exit {
		logger.WithField("content", args).Debug("exit")
		return ctx, cmdOut.String()
	}
	if err != nil {
		logger.WithField("content", args).Errorf("kong parse failed %v", err)
		r.Exit(0)
		var out string
		if !r.silence {
			out = fmt.Sprintf("command parse failed - %v", err)
		}
		if r.groupCode > 0 && r.senderUin > 0 {
			manager := r.l.ensureGroupUXManager()
			policy := manager.Policy(r.groupCode)
			if !manager.AllowParseError(r.groupCode, r.senderUin, name, policy, time.Now()) {
				out = ""
			}
		}
		return nil, out
	}
	return ctx, ""
}

func (r *Runtime) ParseRawSiteAndType(rawSite string, rawType string) (string, concern_type.Type, error) {
	site, ctype, err := concern.ParseRawSiteAndType(rawSite, rawType)
	if err == concern.ErrTypeNotSupported && strings.EqualFold(strings.TrimSpace(rawType), "live") {
		parsedSite, siteErr := concern.ParseRawSite(rawSite)
		if siteErr == nil {
			types, typeErr := concern.GetConcernTypes(parsedSite)
			if typeErr == nil {
				split := types.Split()
				if len(split) == 1 {
					return parsedSite, split[0], nil
				}
			}
		}
	}
	if err == concern.ErrSiteNotSupported {
		err = fmt.Errorf("%v <%v>", err.Error(), rawSite)
	}
	if err == concern.ErrTypeNotSupported {
		err = fmt.Errorf("%v <%v>", err.Error(), rawType)
	}
	return site, ctype, err
}

func (r *Runtime) ParseRawSite(rawSite string) (string, error) {
	site, err := concern.ParseRawSite(rawSite)
	if err == concern.ErrSiteNotSupported {
		err = fmt.Errorf("%v <%v>", err.Error(), rawSite)
	}
	return site, err
}

func NewRuntime(l *Lsp, silence ...bool) *Runtime {
	r := &Runtime{
		bot:    localutils.GetBot(),
		l:      l,
		Parser: parser.NewParser(),
	}
	if len(silence) > 0 {
		r.silence = silence[0]
	}
	return r
}

func (r *Runtime) UseGroupContext(groupCode, senderUin int64) {
	r.groupCode = groupCode
	r.senderUin = senderUin
}
