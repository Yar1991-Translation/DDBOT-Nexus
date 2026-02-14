package lsp

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Mrs4s/MiraiGo/message"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern"
	"github.com/cnxysoft/DDBOT-WSa/lsp/groupux"
	"github.com/cnxysoft/DDBOT-WSa/lsp/mmsg"
	"github.com/cnxysoft/DDBOT-WSa/lsp/template"
	"github.com/cnxysoft/DDBOT-WSa/utils"
	"github.com/cnxysoft/DDBOT-WSa/utils/msgstringer"
	"github.com/sirupsen/logrus"
)

const groupUXDigestTemplate = "notify.group.digest.tmpl"

func (l *Lsp) ConcernNotify() {
	defer func() {
		if err := recover(); err != nil {
			logger.WithField("stack", string(debug.Stack())).Errorf("concern notify recoverd %v", err)
			go l.ConcernNotify()
		}
	}()
	l.wg.Add(1)
	defer l.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.flushGroupUXDigests(time.Now())
		case _inotify, ok := <-l.concernNotify:
			if !ok {
				return
			}
			if _inotify == nil {
				continue
			}
			l.flushGroupUXDigests(time.Now())
			l.handleConcernNotify(_inotify)
		}
	}
}

func (l *Lsp) handleConcernNotify(inotify concern.Notify) {
	target := mmsg.NewGroupTarget(inotify.GetGroupCode())
	nLogger := inotify.Logger()

	if l.LspStateManager.IsMuted(inotify.GetGroupCode(), utils.GetBot().GetUin()) &&
		!l.PermissionStateManager.CheckGroupAdministrator(inotify.GetGroupCode(), utils.GetBot().GetUin()) {
		nLogger.Info("group muted, skip notify")
		return
	}

	c, err := concern.GetConcernBySiteAndType(inotify.Site(), inotify.Type())
	if err != nil {
		nLogger.Errorf("GetConcernBySiteAndType error %v", err)
		return
	}
	cfg := c.GetStateManager().GetGroupConcernConfig(inotify.GetGroupCode(), inotify.GetUid())
	cfg.NotifyBeforeCallback(inotify)

	rawMsg := l.NotifyMessage(inotify)
	if rawMsg == nil {
		nLogger.Warn("notify message is nil")
		return
	}
	m := rawMsg.Clone()

	now := time.Now()
	uxManager := l.ensureGroupUXManager()
	uxPolicy := uxManager.Policy(inotify.GetGroupCode())
	preview := msgstringer.MsgToString(m.ToCombineMessage(target).Elements)
	uid := fmt.Sprint(inotify.GetUid())

	if uxManager.ShouldDropDuplicate(inotify.GetGroupCode(), inotify.Site(), inotify.Type(), uid, preview, uxPolicy, now) {
		nLogger.WithField("groupux", "dedupe").Info("notify dropped by dedupe")
		return
	}

	item := groupux.QuietSummaryItem{
		Site:        inotify.Site(),
		Type:        inotify.Type(),
		UID:         uid,
		Fingerprint: "",
		Message:     preview,
		OccurredAt:  now,
	}

	deliver, summary := uxManager.ApplyQuietHours(inotify.GetGroupCode(), uxPolicy, item, now)
	if summary != nil {
		l.dispatchGroupUXDigest(*summary, nLogger.WithField("groupux", summary.Reason))
	}
	if !deliver {
		nLogger.WithField("groupux", "quiet-hours").Info("notify held by quiet hours")
		return
	}

	if groupux.AggregateTypeEnabled(uxPolicy, inotify.Type()) && uxPolicy.Notify.Aggregate.Window > 0 && uxPolicy.Notify.Aggregate.MaxItems > 1 {
		digest := uxManager.PushAggregate(inotify.GetGroupCode(), inotify.Type(), item, uxPolicy, now)
		if digest == nil {
			nLogger.WithField("groupux", "aggregate").Debug("notify held in aggregate bucket")
			return
		}
		l.dispatchGroupUXDigest(*digest, nLogger.WithField("groupux", "aggregate-flush"))
		return
	}

	atBeforeHook := cfg.AtBeforeHook(inotify)
	if !atBeforeHook.Pass {
		nLogger.WithField("Reason", atBeforeHook.Reason).Debug("notify @at filtered by hook AtBeforeHook")
	} else {
		qqAdmin := l.PermissionStateManager.CheckGroupAdministrator(inotify.GetGroupCode(), utils.GetBot().GetUin())
		checkAtAll := qqAdmin && cfg.GetGroupConcernAt().CheckAtAll(inotify.Type())
		atAllMark := checkAtAll && c.GetStateManager().CheckAndSetAtAllMark(inotify.GetGroupCode(), inotify.GetUid())
		nLogger.WithFields(logrus.Fields{
			"qqAdmin":    qqAdmin,
			"checkAtAll": checkAtAll,
			"atMark":     atAllMark,
		}).Trace("at_all condition")

		if qqAdmin && checkAtAll && atAllMark && uxManager.AllowAtAll(inotify.GetGroupCode(), uxPolicy, now) {
			nLogger = nLogger.WithField("at_all", true)
			newAtAllMsg(m)
		} else {
			if qqAdmin && checkAtAll && atAllMark {
				nLogger.WithField("groupux", "at_all_cooldown").Info("at_all downgraded by cooldown")
			}
			ids := cfg.GetGroupConcernAt().GetAtSomeoneList(inotify.Type())
			nLogger = nLogger.WithField("at_QQ", ids)
			newAtIdsMsg(m, ids)
		}
	}

	l.sendGroupMessageWithLimit(m, target, nLogger, func(msgs []*message.GroupMessage) {
		if len(msgs) > 0 {
			cfg.NotifyAfterCallback(inotify, msgs[0])
		} else {
			cfg.NotifyAfterCallback(inotify, nil)
		}
		if !atBeforeHook.Pass {
			return
		}
		atIdsOnce := false
		for _, sent := range msgs {
			if sent.Id != -1 {
				continue
			}
			elems := utils.MessageFilter(sent.Elements, func(element message.IMessageElement) bool {
				return element.Type() == message.At && element.(*message.AtElement).Target == 0
			})
			if len(elems) == 0 {
				continue
			}

			secondM := mmsg.NewMSGFromGroupMessage(sent)
			secondM.Drop(func(e message.IMessageElement, _ int) bool {
				return e.Type() == message.At && e.(*message.AtElement).Target == 0
			})
			secondRes := l.GM(l.SendMsg(secondM, target))
			if len(secondRes) != 1 {
				panic(fmt.Sprintf("INTERNAL: len(secondRes) is %v", len(secondRes)))
			}
			if secondRes[0].Id == -1 {
				continue
			}
			if !atIdsOnce {
				atIdsOnce = true
			}
		}
		if atIdsOnce {
			ids := cfg.GetGroupConcernAt().GetAtSomeoneList(inotify.Type())
			if len(ids) != 0 {
				nLogger.WithField("at_QQ", ids).Debug("notify atAll failed, try at someone")
				l.SendMsg(newAtIdsMsg(mmsg.NewMSG(), ids), target)
			} else {
				nLogger.Debug("notify atAll failed, at someone not config")
			}
		}
	})
}

func (l *Lsp) sendGroupMessageWithLimit(m *mmsg.MSG, target mmsg.Target, nLogger *logrus.Entry, after func(msgs []*message.GroupMessage)) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	if err := l.msgLimit.Acquire(ctx, 1); err != nil {
		cancel()
		nLogger.WithField("Content", msgstringer.MsgToString(m.Elements())).
			Errorf("notify dropped: message queue blocked over one minute")
		return
	}
	cancel()

	l.notifyWg.Add(1)
	nLogger.Info("notify")
	go func() {
		defer l.notifyWg.Done()
		defer func() {
			l.msgLimit.Release(1)
			if e := recover(); e != nil {
				nLogger.WithField("stack", string(debug.Stack())).
					Errorf("notify panic recovered: %v", e)
			}
		}()
		msgs := l.GM(l.SendMsg(m, target))
		if after != nil {
			after(msgs)
		}
	}()
}

func (l *Lsp) flushGroupUXDigests(now time.Time) {
	manager := l.ensureGroupUXManager()
	for _, digest := range manager.FlushDue(now) {
		if digest.Reason == "aggregate" {
			item := groupux.QuietSummaryItem{
				Site:       "digest",
				Type:       digest.Type,
				UID:        "",
				Message:    fmt.Sprintf("aggregate items=%d", digest.Count),
				OccurredAt: now,
			}
			deliver, summary := manager.ApplyQuietHours(digest.GroupCode, digest.Policy, item, now)
			if summary != nil {
				l.dispatchGroupUXDigest(*summary, logger.WithField("groupux", summary.Reason))
			}
			if !deliver {
				continue
			}
		}
		l.dispatchGroupUXDigest(digest, logger.WithField("groupux", digest.Reason))
	}
}

func (l *Lsp) dispatchGroupUXDigest(digest groupux.PreparedDigest, nLogger *logrus.Entry) {
	if digest.GroupCode == 0 {
		return
	}
	if l.LspStateManager.IsMuted(digest.GroupCode, utils.GetBot().GetUin()) &&
		!l.PermissionStateManager.CheckGroupAdministrator(digest.GroupCode, utils.GetBot().GetUin()) {
		return
	}
	m := l.buildGroupUXDigestMessage(digest)
	if m == nil {
		return
	}
	target := mmsg.NewGroupTarget(digest.GroupCode)
	l.sendGroupMessageWithLimit(m, target, nLogger.WithFields(logrus.Fields{
		"group_code": digest.GroupCode,
		"items":      digest.Count,
		"reason":     digest.Reason,
	}), nil)
}

func (l *Lsp) buildGroupUXDigestMessage(digest groupux.PreparedDigest) *mmsg.MSG {
	maxItems := digest.Policy.Notify.QuietHours.SummaryMaxN
	title := "\u7fa4\u6d88\u606f\u6458\u8981"
	if digest.Reason == "aggregate" {
		title = "\u6d88\u606f\u805a\u5408"
		maxItems = digest.Policy.Notify.Aggregate.MaxItems
	}
	if maxItems <= 0 {
		maxItems = 8
	}
	items, remain := groupux.BuildDigestRenderItems(digest.Items, maxItems, time.Local)
	remain += digest.Dropped
	data := map[string]interface{}{
		"title":        title,
		"reason":       digest.Reason,
		"group_code":   digest.GroupCode,
		"count":        digest.Count,
		"items":        items,
		"remain":       remain,
		"generated_at": time.Now().Format("2006-01-02 15:04:05"),
	}
	msg, err := template.LoadAndExec(groupUXDigestTemplate, data)
	if err != nil {
		logger.WithError(err).Error("render group ux digest template failed")
		fallback := mmsg.NewMSG()
		fallback.Textf("[%s] \u5171 %d \u6761\n", title, digest.Count)
		for _, item := range items {
			fallback.Textf("%d. [%s/%s] %s\n", item.Index, item.Site, item.Type, item.Preview)
		}
		if remain > 0 {
			fallback.Textf("\u5176\u4f59 %d \u6761\n", remain)
		}
		return fallback
	}
	return msg
}

func (l *Lsp) NotifyMessage(inotify concern.Notify) *mmsg.MSG {
	return inotify.ToMessage()
}

func newAtAllMsg(m *mmsg.MSG) *mmsg.MSG {
	return m.AtAll(true)
}

func newAtIdsMsg(m *mmsg.MSG, ids []int64) *mmsg.MSG {
	if len(ids) > 0 {
		m.Cut()
		for _, id := range ids {
			m.At(id)
		}
	}
	return m
}
