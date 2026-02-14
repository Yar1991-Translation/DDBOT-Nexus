package lsp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	localutils "github.com/cnxysoft/DDBOT-WSa/utils"
	"github.com/sirupsen/logrus"
)

const configChatUsage = "config chat <show|aggregate|dedupe_ttl|aggregate_window|at_all_cooldown|quiet_hours|reset>"

func (lgc *LspGroupCommand) handleConfigChatCommand(log *logrus.Entry) bool {
	args := lgc.GetArgs()
	if len(args) == 0 || !strings.EqualFold(strings.TrimSpace(args[0]), "chat") {
		return false
	}

	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(strings.TrimSpace(args[1]))
	}
	log = log.WithField("sub_command", "chat").WithField("chat_sub_command", sub)
	dispatchConfigChatCommand(lgc.NewMessageContext(log), lgc.groupCode(), args[1:])
	return true
}

func (c *LspPrivateCommand) handleConfigChatCommand(log *logrus.Entry) bool {
	groupCode, args, err := extractPrivateConfigGroupArg(c.GetArgs())
	if err != nil {
		c.textReply(fmt.Sprintf("参数错误 - %v", err))
		return true
	}
	if len(args) == 0 || !strings.EqualFold(strings.TrimSpace(args[0]), "chat") {
		return false
	}
	if err := c.checkGroupCode(groupCode); err != nil {
		c.textReply(err.Error())
		return true
	}

	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(strings.TrimSpace(args[1]))
	}
	log = log.WithFields(localutils.GroupLogFields(groupCode)).
		WithField("sub_command", "chat").
		WithField("chat_sub_command", sub)

	dispatchConfigChatCommand(c.NewMessageContext(log), groupCode, args[1:])
	return true
}

func extractPrivateConfigGroupArg(args []string) (int64, []string, error) {
	var (
		groupCode int64
		cleaned   = make([]string, 0, len(args))
	)
	for i := 0; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		switch {
		case token == "-g" || token == "--group":
			if i+1 >= len(args) {
				return 0, nil, fmt.Errorf("缺少 -g 对应的群号")
			}
			value := strings.TrimSpace(args[i+1])
			code, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, nil, fmt.Errorf("群号格式错误: %s", value)
			}
			groupCode = code
			i++
		case strings.HasPrefix(token, "-g="):
			value := strings.TrimSpace(strings.TrimPrefix(token, "-g="))
			code, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, nil, fmt.Errorf("群号格式错误: %s", value)
			}
			groupCode = code
		case strings.HasPrefix(token, "--group="):
			value := strings.TrimSpace(strings.TrimPrefix(token, "--group="))
			code, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, nil, fmt.Errorf("群号格式错误: %s", value)
			}
			groupCode = code
		default:
			cleaned = append(cleaned, args[i])
		}
	}
	return groupCode, cleaned, nil
}

func dispatchConfigChatCommand(ctx *MessageContext, groupCode int64, args []string) {
	if len(args) == 0 {
		ctx.TextReply("参数错误 - 缺少 chat 子命令")
		ctx.TextReply("用法: " + configChatUsage)
		return
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "show":
		IConfigChatShow(ctx, groupCode)
	case "aggregate":
		if len(args) < 2 {
			ctx.TextReply("参数错误 - aggregate 需要 on/off")
			return
		}
		on, err := parseOnOff(args[1])
		if err != nil {
			ctx.TextReply("参数错误 - aggregate 仅支持 on/off")
			return
		}
		IConfigChatAggregate(ctx, groupCode, on)
	case "dedupe_ttl":
		if len(args) < 2 {
			ctx.TextReply("参数错误 - dedupe_ttl 需要 duration")
			return
		}
		d, err := time.ParseDuration(strings.TrimSpace(args[1]))
		if err != nil {
			ctx.TextReply(fmt.Sprintf("参数错误 - dedupe_ttl 无法解析: %v", err))
			return
		}
		IConfigChatDedupeTTL(ctx, groupCode, d)
	case "aggregate_window":
		if len(args) < 2 {
			ctx.TextReply("参数错误 - aggregate_window 需要 duration")
			return
		}
		d, err := time.ParseDuration(strings.TrimSpace(args[1]))
		if err != nil {
			ctx.TextReply(fmt.Sprintf("参数错误 - aggregate_window 无法解析: %v", err))
			return
		}
		IConfigChatAggregateWindow(ctx, groupCode, d)
	case "at_all_cooldown":
		if len(args) < 2 {
			ctx.TextReply("参数错误 - at_all_cooldown 需要 duration")
			return
		}
		d, err := time.ParseDuration(strings.TrimSpace(args[1]))
		if err != nil {
			ctx.TextReply(fmt.Sprintf("参数错误 - at_all_cooldown 无法解析: %v", err))
			return
		}
		IConfigChatAtAllCooldown(ctx, groupCode, d)
	case "quiet_hours":
		if len(args) < 2 {
			ctx.TextReply("参数错误 - quiet_hours 需要 on/off")
			return
		}
		on, err := parseOnOff(args[1])
		if err != nil {
			ctx.TextReply("参数错误 - quiet_hours 仅支持 on/off")
			return
		}
		start, end, summary, err := parseQuietHoursArgs(args[2:])
		if err != nil {
			ctx.TextReply(fmt.Sprintf("参数错误 - %v", err))
			return
		}
		IConfigChatQuietHours(ctx, groupCode, on, start, end, summary)
	case "reset":
		IConfigChatReset(ctx, groupCode)
	default:
		ctx.TextReply(fmt.Sprintf("参数错误 - 未知 chat 子命令: %s", sub))
		ctx.TextReply("用法: " + configChatUsage)
	}
}

func parseOnOff(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid switch")
	}
}

func parseQuietHoursArgs(args []string) (string, string, *bool, error) {
	var (
		start   string
		end     string
		summary *bool
	)

	for i := 0; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		switch {
		case token == "--start":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("--start 缺少时间值")
			}
			start = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(token, "--start="):
			start = strings.TrimSpace(strings.TrimPrefix(token, "--start="))
		case token == "--end":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("--end 缺少时间值")
			}
			end = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(token, "--end="):
			end = strings.TrimSpace(strings.TrimPrefix(token, "--end="))
		case token == "--summary":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("--summary 缺少 on/off")
			}
			value, err := parseOnOff(args[i+1])
			if err != nil {
				return "", "", nil, fmt.Errorf("--summary 仅支持 on/off")
			}
			v := value
			summary = &v
			i++
		case strings.HasPrefix(token, "--summary="):
			raw := strings.TrimSpace(strings.TrimPrefix(token, "--summary="))
			value, err := parseOnOff(raw)
			if err != nil {
				return "", "", nil, fmt.Errorf("--summary 仅支持 on/off")
			}
			v := value
			summary = &v
		default:
			return "", "", nil, fmt.Errorf("不支持的参数: %s", token)
		}
	}
	return start, end, summary, nil
}
