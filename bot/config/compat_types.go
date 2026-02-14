package config

import (
	"fmt"
	"path/filepath"
	"sort"
)

type ValueSource struct {
	Layer string
	File  string
	Line  int
	Key   string
}

func (s ValueSource) ValueSourceText() string {
	if s.File == "" {
		return "unknown:0"
	}
	file := filepath.Base(s.File)
	if s.Line <= 0 {
		return fmt.Sprintf("%s:0", file)
	}
	return fmt.Sprintf("%s:%d", file, s.Line)
}

type ConfigCanonical struct {
	Values  map[string]interface{}
	Sources map[string]ValueSource
}

func newCanonical() ConfigCanonical {
	return ConfigCanonical{
		Values:  make(map[string]interface{}),
		Sources: make(map[string]ValueSource),
	}
}

func (c *ConfigCanonical) Set(key string, value interface{}, src ValueSource, overwrite bool) {
	if !overwrite {
		if _, ok := c.Values[key]; ok {
			return
		}
	}
	c.Values[key] = value
	c.Sources[key] = src
}

func (c ConfigCanonical) SortedKeys() []string {
	keys := make([]string, 0, len(c.Values))
	for key := range c.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func MergeCanonical(base ConfigCanonical, overlay ConfigCanonical) ConfigCanonical {
	merged := newCanonical()
	for _, key := range base.SortedKeys() {
		merged.Values[key] = base.Values[key]
		merged.Sources[key] = base.Sources[key]
	}
	for _, key := range overlay.SortedKeys() {
		merged.Values[key] = overlay.Values[key]
		merged.Sources[key] = overlay.Sources[key]
	}
	return merged
}

type DeprecatedConfigWarning struct {
	OldKey      string
	NewKey      string
	ValueSource string
}

type UnknownV2Field struct {
	Key         string
	ValueSource string
}

type Diagnostics struct {
	Deprecated []DeprecatedConfigWarning
	UnknownV2  []UnknownV2Field
}

func (d *Diagnostics) addDeprecated(oldKey, newKey string, src ValueSource) {
	for _, item := range d.Deprecated {
		if item.OldKey == oldKey {
			return
		}
	}
	d.Deprecated = append(d.Deprecated, DeprecatedConfigWarning{
		OldKey:      oldKey,
		NewKey:      newKey,
		ValueSource: src.ValueSourceText(),
	})
}

func (d *Diagnostics) addUnknownV2(key string, src ValueSource) {
	d.UnknownV2 = append(d.UnknownV2, UnknownV2Field{
		Key:         key,
		ValueSource: src.ValueSourceText(),
	})
}

type resolvedConfigPaths struct {
	Name       string
	Type       string
	SearchPath []string
	LegacyPath string
	V2Path     string
}

func (r resolvedConfigPaths) legacyFileName() string {
	return fmt.Sprintf("%s.%s", r.Name, r.Type)
}

func (r resolvedConfigPaths) v2FileName() string {
	return fmt.Sprintf("%s.v2.%s", r.Name, r.Type)
}

func (r resolvedConfigPaths) PreferredWritePath() string {
	if r.LegacyPath != "" {
		return r.LegacyPath
	}
	if r.V2Path != "" {
		return filepath.Join(filepath.Dir(r.V2Path), r.legacyFileName())
	}
	if len(r.SearchPath) > 0 {
		return filepath.Join(r.SearchPath[0], r.legacyFileName())
	}
	return r.legacyFileName()
}

var defaultsV1Keys = map[string]interface{}{
	"bot.commandPrefix":                         "/",
	"dispatch.largeNotifyLimit":                 50,
	"groupUX.command.conciseReply":              true,
	"groupUX.command.groupUnknownTips":          false,
	"groupUX.command.parseErrorCooldown":        "20s",
	"groupUX.enabled":                           true,
	"groupUX.notify.aggregate.enabled":          true,
	"groupUX.notify.aggregate.maxItems":         5,
	"groupUX.notify.aggregate.types":            []string{"news"},
	"groupUX.notify.aggregate.window":           "45s",
	"groupUX.notify.atAllCooldown":              "30m",
	"groupUX.notify.dedupeTTL":                  "90s",
	"groupUX.notify.quietHours.enabled":         false,
	"groupUX.notify.quietHours.end":             "07:00",
	"groupUX.notify.quietHours.start":           "23:00",
	"groupUX.notify.quietHours.summaryMaxItems": 8,
	"groupUX.notify.quietHours.summaryOnExit":   true,
	"notify.parallel":                           1,
}

var oldToNewKeyMapping = map[string]string{
	"acfun.interval":                            "providers.acfun.interval",
	"autoreply.group.command":                   "templates.autoreply.group.command",
	"autoreply.private.command":                 "templates.autoreply.private.command",
	"bilibili.QRLogin":                          "providers.bilibili.qrLogin",
	"bilibili.SESSDATA":                         "providers.bilibili.sessdata",
	"bilibili.account":                          "providers.bilibili.account",
	"bilibili.autoParsePosts":                   "providers.bilibili.autoParsePosts",
	"bilibili.bili_jct":                         "providers.bilibili.biliJct",
	"bilibili.disableSub":                       "providers.bilibili.disableSub",
	"bilibili.hiddenSub":                        "providers.bilibili.hiddenSub",
	"bilibili.imageMergeMode":                   "providers.bilibili.imageMergeMode",
	"bilibili.interval":                         "providers.bilibili.interval",
	"bilibili.minFollowerCap":                   "providers.bilibili.minFollowerCap",
	"bilibili.onlyOnlineNotify":                 "providers.bilibili.onlyOnlineNotify",
	"bilibili.password":                         "providers.bilibili.password",
	"bilibili.unsub":                            "providers.bilibili.unsub",
	"bot.account":                               "bot.account",
	"bot.commandPrefix":                         "bot.commandPrefix",
	"bot.offlineQueue.enable":                   "bot.offlineQueue.enable",
	"bot.offlineQueue.expire":                   "bot.offlineQueue.expire",
	"bot.onDisconnected":                        "bot.onDisconnected",
	"bot.onJoinGroup.rename":                    "bot.onJoinGroup.rename",
	"bot.password":                              "bot.password",
	"bot.sendFailureReminder.enable":            "bot.sendFailureReminder.enable",
	"bot.sendFailureReminder.times":             "bot.sendFailureReminder.times",
	"concern.emitInterval":                      "notify.concern.emitInterval",
	"cronjob":                                   "cronjob",
	"customCommandPrefix":                       "customCommandPrefix",
	"debug.group":                               "features.debug.group",
	"debug.uin":                                 "features.debug.uin",
	"dispatch.largeNotifyLimit":                 "notify.dispatch.largeNotifyLimit",
	"douyin.acNonce":                            "providers.douyin.acNonce",
	"douyin.acSignature":                        "providers.douyin.acSignature",
	"douyin.interval":                           "providers.douyin.interval",
	"douyin.userAgent":                          "providers.douyin.userAgent",
	"imagePool.type":                            "media.imagePool.type",
	"localPool.imageDir":                        "media.localPool.imageDir",
	"localProxyPool.mainland":                   "network.proxy.localPool.mainland",
	"localProxyPool.oversea":                    "network.proxy.localPool.oversea",
	"logLevel":                                  "logging.level",
	"loliconPool.apikey":                        "media.loliconPool.apiKey",
	"loliconPool.cacheMax":                      "media.loliconPool.cacheMax",
	"loliconPool.cacheMin":                      "media.loliconPool.cacheMin",
	"loliconPool.proxy":                         "media.loliconPool.proxy",
	"message-marker.disable":                    "features.messageMarker.disable",
	"notify.parallel":                           "notify.parallel",
	"proxy.type":                                "network.proxy.type",
	"pyProxyPool.host":                          "network.proxy.pyPool.host",
	"qq-logs.enable":                            "logging.qqLogs.enable",
	"qq-logs.enabled":                           "logging.qqLogs.enabled",
	"reloadDelay.enable":                        "runtime.reloadDelay.enable",
	"reloadDelay.time":                          "runtime.reloadDelay.time",
	"sign-server":                               "transport.signServer",
	"template.enable":                           "templates.engine.enable",
	"twitcasting":                               "providers.twitcasting",
	"twitcasting.broadcaster.created":           "providers.twitcasting.broadcaster.created",
	"twitcasting.broadcaster.image":             "providers.twitcasting.broadcaster.image",
	"twitcasting.broadcaster.title":             "providers.twitcasting.broadcaster.title",
	"twitcasting.clientId":                      "providers.twitcasting.clientId",
	"twitcasting.clientSecret":                  "providers.twitcasting.clientSecret",
	"twitcasting.nameStrategy":                  "providers.twitcasting.nameStrategy",
	"twitter.BaseUrl":                           "providers.twitter.baseUrl",
	"twitter.interval":                          "providers.twitter.interval",
	"twitter.userAgent":                         "providers.twitter.userAgent",
	"groupUX.command.conciseReply":              "features.groupUx.command.conciseReply",
	"groupUX.command.groupUnknownTips":          "features.groupUx.command.groupUnknownTips",
	"groupUX.command.parseErrorCooldown":        "features.groupUx.command.parseErrorCooldown",
	"groupUX.enabled":                           "features.groupUx.enabled",
	"groupUX.notify.aggregate.enabled":          "notify.groupUx.aggregate.enabled",
	"groupUX.notify.aggregate.maxItems":         "notify.groupUx.aggregate.maxItems",
	"groupUX.notify.aggregate.types":            "notify.groupUx.aggregate.types",
	"groupUX.notify.aggregate.window":           "notify.groupUx.aggregate.window",
	"groupUX.notify.atAllCooldown":              "notify.groupUx.atAllCooldown",
	"groupUX.notify.dedupeTTL":                  "notify.groupUx.dedupeTTL",
	"groupUX.notify.quietHours.enabled":         "notify.groupUx.quietHours.enabled",
	"groupUX.notify.quietHours.end":             "notify.groupUx.quietHours.end",
	"groupUX.notify.quietHours.start":           "notify.groupUx.quietHours.start",
	"groupUX.notify.quietHours.summaryMaxItems": "notify.groupUx.quietHours.summaryMaxItems",
	"groupUX.notify.quietHours.summaryOnExit":   "notify.groupUx.quietHours.summaryOnExit",
	"websocket.mode":                            "transport.websocket.mode",
	"websocket.token":                           "transport.websocket.token",
	"websocket.ws-reverse":                      "transport.websocket.wsReverse",
	"websocket.ws-server":                       "transport.websocket.wsServer",
}

var newToOldKeyMapping = func() map[string]string {
	result := make(map[string]string, len(oldToNewKeyMapping))
	for oldKey, newKey := range oldToNewKeyMapping {
		if _, exists := result[newKey]; !exists {
			result[newKey] = oldKey
		}
	}
	return result
}()

// v2OnlyPassthroughPrefixes are recognized v2 keys that should be loaded
// directly without legacy mapping, and should not be flagged as unknown.
var v2OnlyPassthroughPrefixes = []string{
	"providers.roblox",
}
