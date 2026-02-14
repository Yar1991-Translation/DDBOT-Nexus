package cfg

import (
	"errors"
	"github.com/Sora233/MiraiGo-Template/config"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
	"github.com/cnxysoft/DDBOT-WSa/lsp/groupux"
	"go.uber.org/atomic"
	"strings"
	"time"
)

func MatchCmdWithPrefix(cmd string) (prefix string, command string, err error) {
	var customPrefixCfg = GetCustomCommandPrefix()
	if customPrefixCfg != nil {
		for k, v := range customPrefixCfg {
			if v+k == cmd {
				return v, k, nil
			}
		}
	}
	commonPrefix := GetCommandPrefix()
	if strings.HasPrefix(cmd, commonPrefix) {
		return commonPrefix, strings.TrimPrefix(cmd, commonPrefix), nil
	}
	return "", "", errors.New("match failed")
}

func GetCommandPrefix(commands ...string) string {
	if len(commands) > 0 {
		var customPrefixCfg = GetCustomCommandPrefix()
		if customPrefixCfg != nil {
			if prefix, found := customPrefixCfg[commands[0]]; found {
				return prefix
			}
		}
	}
	prefix := strings.TrimSpace(config.GlobalConfig.GetString("bot.commandPrefix"))
	if len(prefix) == 0 {
		prefix = "/"
	}
	return prefix
}

var customCommandPrefixAtomic atomic.Value

// ReloadCustomCommandPrefix TODO wtf
func ReloadCustomCommandPrefix() {
	result := config.GlobalConfig.GetStringMapString("customCommandPrefix")
	if len(result) == 0 {
		result = config.GlobalConfig.GetStringMapString("customcommandprefix")
	}
	if result == nil {
		result = make(map[string]string)
	}
	customCommandPrefixAtomic.Store(result)
}

func GetCustomCommandPrefix() map[string]string {
	var m = customCommandPrefixAtomic.Load()
	if m == nil {
		m = make(map[string]string)
	}
	return m.(map[string]string)
}

func GetEmitInterval() time.Duration {
	return config.GlobalConfig.GetDuration("concern.emitInterval")
}

func GetLargeNotifyLimit() int {
	var limit = config.GlobalConfig.GetInt("dispatch.largeNotifyLimit")
	if limit <= 0 {
		limit = 50
	}
	return limit
}

type CronJob struct {
	Cron         string `yaml:"cron"`
	TemplateName string `yaml:"templateName"`
	Target       struct {
		Group   []int64 `yaml:"group"`
		Private []int64 `yaml:"private"`
	} `yaml:"target"`
}

func GetCronJob() []*CronJob {
	var result []*CronJob
	if err := config.GlobalConfig.UnmarshalKey("cronjob", &result); err != nil {
		logger.Errorf("GetCronJob UnmarshalKey <cronjob> error %v", err)
		return nil
	}
	return result
}

func GetTemplateEnabled() bool {
	return config.GlobalConfig.GetBool("template.enable")
}

func GetCustomGroupCommand() []string {
	return config.GlobalConfig.GetStringSlice("autoreply.group.command")
}

func GetCustomPrivateCommand() []string {
	return config.GlobalConfig.GetStringSlice("autoreply.private.command")
}

func GetBilibiliMinFollowerCap() int {
	return config.GlobalConfig.GetInt("bilibili.minFollowerCap")
}

func GetBilibiliDisableSub() bool {
	return config.GlobalConfig.GetBool("bilibili.disableSub")
}

func GetBilibiliHiddenSub() bool {
	return config.GlobalConfig.GetBool("bilibili.hiddenSub")
}

func GetBilibiliUnsub() bool {
	return config.GlobalConfig.GetBool("bilibili.unsub")
}

func GetNotifyParallel() int {
	var parallel = config.GlobalConfig.GetInt("notify.parallel")
	if parallel <= 0 {
		parallel = 1
	}
	return parallel
}

func GetBilibiliOnlyOnlineNotify() bool {
	return config.GlobalConfig.GetBool("bilibili.onlyOnlineNotify")
}

func GetGroupUXPolicy() groupux.GroupUXPolicy {
	policy := groupux.GroupUXPolicy{
		Enabled: getBoolDefault("groupUX.enabled", true),
		Command: groupux.GroupUXCommandPolicy{
			ConciseReply:       getBoolDefault("groupUX.command.conciseReply", true),
			ParseErrorCooldown: getDurationDefault("groupUX.command.parseErrorCooldown", 20*time.Second),
			GroupUnknownTips:   getBoolDefault("groupUX.command.groupUnknownTips", false),
		},
		Notify: groupux.GroupUXNotifyPolicy{
			DedupeTTL: getDurationDefault("groupUX.notify.dedupeTTL", 90*time.Second),
			Aggregate: groupux.GroupUXAggregatePolicy{
				Enabled:  getBoolDefault("groupUX.notify.aggregate.enabled", true),
				Window:   getDurationDefault("groupUX.notify.aggregate.window", 45*time.Second),
				MaxItems: getIntDefault("groupUX.notify.aggregate.maxItems", 5),
				Types:    normalizeAggregateTypes(config.GlobalConfig.GetStringSlice("groupUX.notify.aggregate.types")),
			},
			AtAllCooldown: getDurationDefault("groupUX.notify.atAllCooldown", 30*time.Minute),
			QuietHours: groupux.GroupUXQuietHoursPolicy{
				Enabled:       getBoolDefault("groupUX.notify.quietHours.enabled", false),
				Start:         getTimeWindowDefault("groupUX.notify.quietHours.start", "23:00"),
				End:           getTimeWindowDefault("groupUX.notify.quietHours.end", "07:00"),
				SummaryOnExit: getBoolDefault("groupUX.notify.quietHours.summaryOnExit", true),
				SummaryMaxN:   getIntDefault("groupUX.notify.quietHours.summaryMaxItems", 8),
			},
		},
	}

	if policy.Command.ParseErrorCooldown < 0 {
		policy.Command.ParseErrorCooldown = 0
	}
	if policy.Notify.DedupeTTL < 0 {
		policy.Notify.DedupeTTL = 0
	}
	if policy.Notify.Aggregate.Window < 0 {
		policy.Notify.Aggregate.Window = 0
	}
	if policy.Notify.Aggregate.MaxItems <= 0 {
		policy.Notify.Aggregate.MaxItems = 5
	}
	if len(policy.Notify.Aggregate.Types) == 0 {
		policy.Notify.Aggregate.Types = []concern_type.Type{concern_type.Type("news")}
	}
	if policy.Notify.AtAllCooldown < 0 {
		policy.Notify.AtAllCooldown = 0
	}
	if policy.Notify.QuietHours.SummaryMaxN <= 0 {
		policy.Notify.QuietHours.SummaryMaxN = 8
	}
	return policy
}

func normalizeAggregateTypes(raw []string) []concern_type.Type {
	if len(raw) == 0 {
		return []concern_type.Type{concern_type.Type("news")}
	}
	seen := make(map[string]struct{}, len(raw))
	types := make([]concern_type.Type, 0, len(raw))
	for _, item := range raw {
		tp := strings.TrimSpace(item)
		if tp == "" {
			continue
		}
		tp = strings.ToLower(tp)
		if _, ok := seen[tp]; ok {
			continue
		}
		seen[tp] = struct{}{}
		types = append(types, concern_type.Type(tp))
	}
	return types
}

func getBoolDefault(key string, fallback bool) bool {
	if !config.GlobalConfig.IsSet(key) {
		return fallback
	}
	return config.GlobalConfig.GetBool(key)
}

func getIntDefault(key string, fallback int) int {
	if !config.GlobalConfig.IsSet(key) {
		return fallback
	}
	return config.GlobalConfig.GetInt(key)
}

func getDurationDefault(key string, fallback time.Duration) time.Duration {
	if !config.GlobalConfig.IsSet(key) {
		return fallback
	}
	raw := strings.TrimSpace(config.GlobalConfig.GetString(key))
	if raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err == nil {
			return parsed
		}
	}
	value := config.GlobalConfig.GetDuration(key)
	if value == 0 {
		return fallback
	}
	return value
}

func getTimeWindowDefault(key, fallback string) string {
	raw := strings.TrimSpace(config.GlobalConfig.GetString(key))
	if raw == "" {
		raw = fallback
	}
	if _, err := time.Parse("15:04", raw); err != nil {
		return fallback
	}
	return raw
}
