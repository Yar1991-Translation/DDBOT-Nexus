package groupux

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
)

type QuietSummaryItem struct {
	Site        string
	Type        concern_type.Type
	UID         string
	Fingerprint string
	Message     string
	OccurredAt  time.Time
}

type DigestBucket struct {
	GroupCode int64
	Type      concern_type.Type
	Items     []QuietSummaryItem
	FlushAt   time.Time
}

type PreparedDigest struct {
	GroupCode int64
	Type      concern_type.Type
	Policy    GroupUXPolicy
	Items     []QuietSummaryItem
	Count     int
	Dropped   int
	Reason    string
}

func (p GroupUXPolicy) Clone() GroupUXPolicy {
	cloned := p
	if len(p.Notify.Aggregate.Types) > 0 {
		cloned.Notify.Aggregate.Types = append([]concern_type.Type{}, p.Notify.Aggregate.Types...)
	}
	return cloned
}

func MergePolicy(base GroupUXPolicy, override GroupUXOverride) GroupUXPolicy {
	merged := base.Clone()
	if override.Empty() {
		return sanitizePolicy(merged)
	}

	if value, ok := override.GetBool(OverrideEnabled); ok {
		merged.Enabled = value
	}
	if value, ok := override.GetBool(OverrideCommandConciseReply); ok {
		merged.Command.ConciseReply = value
	}
	if value, ok := override.GetDuration(OverrideCommandParseErrorCooldown); ok {
		merged.Command.ParseErrorCooldown = value
	}
	if value, ok := override.GetBool(OverrideCommandGroupUnknownTips); ok {
		merged.Command.GroupUnknownTips = value
	}
	if value, ok := override.GetDuration(OverrideNotifyDedupeTTL); ok {
		merged.Notify.DedupeTTL = value
	}
	if value, ok := override.GetBool(OverrideNotifyAggregateEnabled); ok {
		merged.Notify.Aggregate.Enabled = value
	}
	if value, ok := override.GetDuration(OverrideNotifyAggregateWindow); ok {
		merged.Notify.Aggregate.Window = value
	}
	if value, ok := override.GetInt(OverrideNotifyAggregateMaxItems); ok {
		merged.Notify.Aggregate.MaxItems = value
	}
	if value, ok := override.GetString(OverrideNotifyAggregateTypes); ok {
		merged.Notify.Aggregate.Types = parseAggregateTypes(value)
	}
	if value, ok := override.GetDuration(OverrideNotifyAtAllCooldown); ok {
		merged.Notify.AtAllCooldown = value
	}
	if value, ok := override.GetBool(OverrideNotifyQuietHoursEnabled); ok {
		merged.Notify.QuietHours.Enabled = value
	}
	if value, ok := override.GetString(OverrideNotifyQuietHoursStart); ok {
		merged.Notify.QuietHours.Start = value
	}
	if value, ok := override.GetString(OverrideNotifyQuietHoursEnd); ok {
		merged.Notify.QuietHours.End = value
	}
	if value, ok := override.GetBool(OverrideNotifyQuietHoursSummaryOn); ok {
		merged.Notify.QuietHours.SummaryOnExit = value
	}
	if value, ok := override.GetInt(OverrideNotifyQuietHoursSummaryN); ok {
		merged.Notify.QuietHours.SummaryMaxN = value
	}

	return sanitizePolicy(merged)
}

func sanitizePolicy(p GroupUXPolicy) GroupUXPolicy {
	if p.Command.ParseErrorCooldown < 0 {
		p.Command.ParseErrorCooldown = 0
	}
	if p.Notify.DedupeTTL < 0 {
		p.Notify.DedupeTTL = 0
	}
	if p.Notify.Aggregate.Window < 0 {
		p.Notify.Aggregate.Window = 0
	}
	if p.Notify.Aggregate.MaxItems <= 0 {
		p.Notify.Aggregate.MaxItems = 1
	}
	if len(p.Notify.Aggregate.Types) == 0 {
		p.Notify.Aggregate.Types = []concern_type.Type{concern_type.Type("news")}
	}
	if p.Notify.AtAllCooldown < 0 {
		p.Notify.AtAllCooldown = 0
	}
	if p.Notify.QuietHours.SummaryMaxN <= 0 {
		p.Notify.QuietHours.SummaryMaxN = 1
	}
	if _, err := parseHHMM(p.Notify.QuietHours.Start); err != nil {
		p.Notify.QuietHours.Start = "23:00"
	}
	if _, err := parseHHMM(p.Notify.QuietHours.End); err != nil {
		p.Notify.QuietHours.End = "07:00"
	}
	return p
}

func parseAggregateTypes(raw string) []concern_type.Type {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []concern_type.Type{concern_type.Type("news")}
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '/', '|', ' ':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return []concern_type.Type{concern_type.Type("news")}
	}
	seen := make(map[string]struct{}, len(parts))
	result := make([]concern_type.Type, 0, len(parts))
	for _, item := range parts {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, concern_type.Type(item))
	}
	if len(result) == 0 {
		return []concern_type.Type{concern_type.Type("news")}
	}
	return result
}

func AggregateTypesToString(types []concern_type.Type) string {
	if len(types) == 0 {
		return "news"
	}
	parts := make([]string, 0, len(types))
	seen := make(map[string]struct{}, len(types))
	for _, tp := range types {
		name := strings.ToLower(strings.TrimSpace(tp.String()))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		parts = append(parts, name)
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "news"
	}
	return strings.Join(parts, ",")
}

func AggregateTypeEnabled(policy GroupUXPolicy, tp concern_type.Type) bool {
	if !policy.Enabled || !policy.Notify.Aggregate.Enabled {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(tp.String()))
	if name == "" {
		return false
	}
	for _, allow := range policy.Notify.Aggregate.Types {
		if strings.EqualFold(allow.String(), name) {
			return true
		}
	}
	return false
}

func InQuietHours(policy GroupUXPolicy, now time.Time) bool {
	if !policy.Enabled || !policy.Notify.QuietHours.Enabled {
		return false
	}
	start, err := parseHHMM(policy.Notify.QuietHours.Start)
	if err != nil {
		return false
	}
	end, err := parseHHMM(policy.Notify.QuietHours.End)
	if err != nil {
		return false
	}
	current := now.Hour()*60 + now.Minute()
	if start == end {
		return true
	}
	if start < end {
		return current >= start && current < end
	}
	return current >= start || current < end
}

func parseHHMM(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return 0, strconv.ErrSyntax
	}
	hh, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	mm, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, strconv.ErrRange
	}
	return hh*60 + mm, nil
}
