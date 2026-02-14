package lsp

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cnxysoft/DDBOT-WSa/lsp/groupux"
	"github.com/cnxysoft/DDBOT-WSa/lsp/permission"
)

func IConfigChatShow(c *MessageContext, groupCode int64) {
	if err := configCmdGroupCommonCheck(c, groupCode); err != nil {
		if permission.IsPermissionError(err) {
			return
		}
		c.TextReply(fmt.Sprintf("失败 - %v", err))
		return
	}

	manager := c.Lsp.ensureGroupUXManager()
	policy := manager.Policy(groupCode)
	override, err := manager.GetGroupOverride(groupCode)
	if err != nil {
		c.GetLog().WithError(err).Error("load group ux override failed")
		c.TextReply("失败 - 读取 chat 配置失败")
		return
	}

	lines := []string{
		"成功 - 当前 chat 配置",
		fmt.Sprintf("enabled: %t", policy.Enabled),
		fmt.Sprintf("command.concise_reply: %t", policy.Command.ConciseReply),
		fmt.Sprintf("command.parse_error_cooldown: %s", policy.Command.ParseErrorCooldown),
		fmt.Sprintf("command.group_unknown_tips: %t", policy.Command.GroupUnknownTips),
		fmt.Sprintf("notify.dedupe_ttl: %s", policy.Notify.DedupeTTL),
		fmt.Sprintf("notify.aggregate.enabled: %t", policy.Notify.Aggregate.Enabled),
		fmt.Sprintf("notify.aggregate.window: %s", policy.Notify.Aggregate.Window),
		fmt.Sprintf("notify.aggregate.max_items: %d", policy.Notify.Aggregate.MaxItems),
		fmt.Sprintf("notify.aggregate.types: %s", groupux.AggregateTypesToString(policy.Notify.Aggregate.Types)),
		fmt.Sprintf("notify.at_all_cooldown: %s", policy.Notify.AtAllCooldown),
		fmt.Sprintf("notify.quiet_hours.enabled: %t", policy.Notify.QuietHours.Enabled),
		fmt.Sprintf("notify.quiet_hours.start: %s", policy.Notify.QuietHours.Start),
		fmt.Sprintf("notify.quiet_hours.end: %s", policy.Notify.QuietHours.End),
		fmt.Sprintf("notify.quiet_hours.summary_on_exit: %t", policy.Notify.QuietHours.SummaryOnExit),
		fmt.Sprintf("notify.quiet_hours.summary_max_items: %d", policy.Notify.QuietHours.SummaryMaxN),
	}

	if override.Empty() {
		lines = append(lines, "override: (继承全局)")
	} else {
		keys := make([]string, 0, len(override.Values))
		for key := range override.Values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines = append(lines, "override:")
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("- %s = %s", key, override.Values[key]))
		}
	}

	c.TextReply(strings.Join(lines, "\n"))
}

func IConfigChatAggregate(c *MessageContext, groupCode int64, enabled bool) {
	err := iConfigChatApply(c, groupCode, func(o *groupux.GroupUXOverride) error {
		o.SetBool(groupux.OverrideNotifyAggregateEnabled, enabled)
		return nil
	})
	if err != nil {
		iConfigChatReplyErr(c, err)
		return
	}
	c.TextReply(fmt.Sprintf("成功 - aggregate 已设置为 %s", onOff(enabled)))
}

func IConfigChatDedupeTTL(c *MessageContext, groupCode int64, ttl time.Duration) {
	if ttl < 0 {
		c.TextReply("失败 - dedupe_ttl 不能小于 0")
		return
	}
	err := iConfigChatApply(c, groupCode, func(o *groupux.GroupUXOverride) error {
		o.SetDuration(groupux.OverrideNotifyDedupeTTL, ttl)
		return nil
	})
	if err != nil {
		iConfigChatReplyErr(c, err)
		return
	}
	c.TextReply(fmt.Sprintf("成功 - dedupe_ttl 已设置为 %s", ttl))
}

func IConfigChatAggregateWindow(c *MessageContext, groupCode int64, window time.Duration) {
	if window < 0 {
		c.TextReply("失败 - aggregate_window 不能小于 0")
		return
	}
	err := iConfigChatApply(c, groupCode, func(o *groupux.GroupUXOverride) error {
		o.SetDuration(groupux.OverrideNotifyAggregateWindow, window)
		return nil
	})
	if err != nil {
		iConfigChatReplyErr(c, err)
		return
	}
	c.TextReply(fmt.Sprintf("成功 - aggregate_window 已设置为 %s", window))
}

func IConfigChatAtAllCooldown(c *MessageContext, groupCode int64, cooldown time.Duration) {
	if cooldown < 0 {
		c.TextReply("失败 - at_all_cooldown 不能小于 0")
		return
	}
	err := iConfigChatApply(c, groupCode, func(o *groupux.GroupUXOverride) error {
		o.SetDuration(groupux.OverrideNotifyAtAllCooldown, cooldown)
		return nil
	})
	if err != nil {
		iConfigChatReplyErr(c, err)
		return
	}
	c.TextReply(fmt.Sprintf("成功 - at_all_cooldown 已设置为 %s", cooldown))
}

func IConfigChatQuietHours(c *MessageContext, groupCode int64, enabled bool, start, end string, summary *bool) {
	var (
		normalizedStart string
		normalizedEnd   string
		err             error
	)
	if strings.TrimSpace(start) != "" {
		normalizedStart, err = normalizeHHMM(start)
		if err != nil {
			c.TextReply(fmt.Sprintf("失败 - quiet_hours start 格式错误: %s", start))
			return
		}
	}
	if strings.TrimSpace(end) != "" {
		normalizedEnd, err = normalizeHHMM(end)
		if err != nil {
			c.TextReply(fmt.Sprintf("失败 - quiet_hours end 格式错误: %s", end))
			return
		}
	}

	err = iConfigChatApply(c, groupCode, func(o *groupux.GroupUXOverride) error {
		o.SetBool(groupux.OverrideNotifyQuietHoursEnabled, enabled)
		if normalizedStart != "" {
			o.SetString(groupux.OverrideNotifyQuietHoursStart, normalizedStart)
		}
		if normalizedEnd != "" {
			o.SetString(groupux.OverrideNotifyQuietHoursEnd, normalizedEnd)
		}
		if summary != nil {
			o.SetBool(groupux.OverrideNotifyQuietHoursSummaryOn, *summary)
		}
		return nil
	})
	if err != nil {
		iConfigChatReplyErr(c, err)
		return
	}

	msg := fmt.Sprintf("成功 - quiet_hours 已设置为 %s", onOff(enabled))
	if normalizedStart != "" || normalizedEnd != "" || summary != nil {
		extras := make([]string, 0, 3)
		if normalizedStart != "" {
			extras = append(extras, "start="+normalizedStart)
		}
		if normalizedEnd != "" {
			extras = append(extras, "end="+normalizedEnd)
		}
		if summary != nil {
			extras = append(extras, "summary="+onOff(*summary))
		}
		msg += " (" + strings.Join(extras, ", ") + ")"
	}
	c.TextReply(msg)
}

func IConfigChatReset(c *MessageContext, groupCode int64) {
	if err := configCmdGroupCommonCheck(c, groupCode); err != nil {
		if permission.IsPermissionError(err) {
			return
		}
		c.TextReply(fmt.Sprintf("失败 - %v", err))
		return
	}
	manager := c.Lsp.ensureGroupUXManager()
	if err := manager.ResetGroupOverride(groupCode); err != nil {
		c.GetLog().WithError(err).Error("reset group ux override failed")
		c.TextReply("失败 - reset chat 配置失败")
		return
	}
	c.TextReply("成功 - chat 配置已重置为全局默认")
}

func iConfigChatApply(c *MessageContext, groupCode int64, apply func(o *groupux.GroupUXOverride) error) error {
	if err := configCmdGroupCommonCheck(c, groupCode); err != nil {
		return err
	}
	manager := c.Lsp.ensureGroupUXManager()
	override, err := manager.GetGroupOverride(groupCode)
	if err != nil {
		return fmt.Errorf("读取 override 失败: %w", err)
	}
	if apply != nil {
		if err := apply(&override); err != nil {
			return err
		}
	}
	if err := manager.SaveGroupOverride(groupCode, override); err != nil {
		return fmt.Errorf("保存 override 失败: %w", err)
	}
	return nil
}

func iConfigChatReplyErr(c *MessageContext, err error) {
	if permission.IsPermissionError(err) {
		return
	}
	c.TextReply(fmt.Sprintf("失败 - %v", err))
}

func normalizeHHMM(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty")
	}
	t, err := time.Parse("15:04", raw)
	if err != nil {
		return "", err
	}
	return t.Format("15:04"), nil
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}
