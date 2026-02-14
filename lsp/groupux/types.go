package groupux

import (
	"strconv"
	"strings"
	"time"

	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
)

const (
	OverrideEnabled                   = "enabled"
	OverrideCommandConciseReply       = "command.conciseReply"
	OverrideCommandParseErrorCooldown = "command.parseErrorCooldown"
	OverrideCommandGroupUnknownTips   = "command.groupUnknownTips"
	OverrideNotifyDedupeTTL           = "notify.dedupeTTL"
	OverrideNotifyAggregateEnabled    = "notify.aggregate.enabled"
	OverrideNotifyAggregateWindow     = "notify.aggregate.window"
	OverrideNotifyAggregateMaxItems   = "notify.aggregate.maxItems"
	OverrideNotifyAggregateTypes      = "notify.aggregate.types"
	OverrideNotifyAtAllCooldown       = "notify.atAllCooldown"
	OverrideNotifyQuietHoursEnabled   = "notify.quietHours.enabled"
	OverrideNotifyQuietHoursStart     = "notify.quietHours.start"
	OverrideNotifyQuietHoursEnd       = "notify.quietHours.end"
	OverrideNotifyQuietHoursSummaryOn = "notify.quietHours.summaryOnExit"
	OverrideNotifyQuietHoursSummaryN  = "notify.quietHours.summaryMaxItems"
)

var validOverrideKeys = map[string]struct{}{
	OverrideEnabled:                   {},
	OverrideCommandConciseReply:       {},
	OverrideCommandParseErrorCooldown: {},
	OverrideCommandGroupUnknownTips:   {},
	OverrideNotifyDedupeTTL:           {},
	OverrideNotifyAggregateEnabled:    {},
	OverrideNotifyAggregateWindow:     {},
	OverrideNotifyAggregateMaxItems:   {},
	OverrideNotifyAggregateTypes:      {},
	OverrideNotifyAtAllCooldown:       {},
	OverrideNotifyQuietHoursEnabled:   {},
	OverrideNotifyQuietHoursStart:     {},
	OverrideNotifyQuietHoursEnd:       {},
	OverrideNotifyQuietHoursSummaryOn: {},
	OverrideNotifyQuietHoursSummaryN:  {},
}

type GroupUXPolicy struct {
	Enabled bool
	Command GroupUXCommandPolicy
	Notify  GroupUXNotifyPolicy
}

type GroupUXCommandPolicy struct {
	ConciseReply       bool
	ParseErrorCooldown time.Duration
	GroupUnknownTips   bool
}

type GroupUXNotifyPolicy struct {
	DedupeTTL     time.Duration
	Aggregate     GroupUXAggregatePolicy
	AtAllCooldown time.Duration
	QuietHours    GroupUXQuietHoursPolicy
}

type GroupUXAggregatePolicy struct {
	Enabled  bool
	Window   time.Duration
	MaxItems int
	Types    []concern_type.Type
}

type GroupUXQuietHoursPolicy struct {
	Enabled       bool
	Start         string
	End           string
	SummaryOnExit bool
	SummaryMaxN   int
}

type GroupUXOverride struct {
	Values map[string]string `json:"values,omitempty"`
}

func NewGroupUXOverride() GroupUXOverride {
	return GroupUXOverride{Values: map[string]string{}}
}

func (o GroupUXOverride) Clone() GroupUXOverride {
	out := NewGroupUXOverride()
	for key, value := range o.Values {
		out.Values[key] = value
	}
	return out
}

func (o GroupUXOverride) Empty() bool {
	return len(o.Values) == 0
}

func (o *GroupUXOverride) normalize() {
	if o.Values == nil {
		o.Values = make(map[string]string)
	}
}

func (o *GroupUXOverride) SetString(key, value string) {
	o.normalize()
	if _, ok := validOverrideKeys[key]; !ok {
		return
	}
	o.Values[key] = strings.TrimSpace(value)
}

func (o *GroupUXOverride) SetBool(key string, value bool) {
	o.SetString(key, strconv.FormatBool(value))
}

func (o *GroupUXOverride) SetInt(key string, value int) {
	o.SetString(key, strconv.Itoa(value))
}

func (o *GroupUXOverride) SetDuration(key string, value time.Duration) {
	o.SetString(key, value.String())
}

func (o *GroupUXOverride) Delete(key string) {
	if o.Values == nil {
		return
	}
	delete(o.Values, key)
}

func (o GroupUXOverride) GetString(key string) (string, bool) {
	if o.Values == nil {
		return "", false
	}
	value, ok := o.Values[key]
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func (o GroupUXOverride) GetBool(key string) (bool, bool) {
	raw, ok := o.GetString(key)
	if !ok {
		return false, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}
	return value, true
}

func (o GroupUXOverride) GetInt(key string) (int, bool) {
	raw, ok := o.GetString(key)
	if !ok {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (o GroupUXOverride) GetDuration(key string) (time.Duration, bool) {
	raw, ok := o.GetString(key)
	if !ok {
		return 0, false
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

