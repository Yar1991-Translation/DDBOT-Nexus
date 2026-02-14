package lsp

import (
	"github.com/cnxysoft/DDBOT-WSa/lsp/cfg"
	"github.com/cnxysoft/DDBOT-WSa/lsp/groupux"
)

func (l *Lsp) ensureGroupUXManager() *groupux.Manager {
	if l.GroupUXManager == nil {
		l.GroupUXManager = groupux.NewManager(cfg.GetGroupUXPolicy())
	}
	return l.GroupUXManager
}
