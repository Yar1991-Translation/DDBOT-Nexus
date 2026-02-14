package groupux

import (
	"testing"
	"time"

	localdb "github.com/cnxysoft/DDBOT-WSa/lsp/buntdb"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
)

func TestMergePolicyOverride(t *testing.T) {
	base := GroupUXPolicy{
		Enabled: true,
		Command: GroupUXCommandPolicy{
			ConciseReply:       true,
			ParseErrorCooldown: 20 * time.Second,
			GroupUnknownTips:   false,
		},
		Notify: GroupUXNotifyPolicy{
			DedupeTTL: 90 * time.Second,
			Aggregate: GroupUXAggregatePolicy{
				Enabled:  true,
				Window:   45 * time.Second,
				MaxItems: 5,
				Types:    []concern_type.Type{concern_type.Type("news")},
			},
			AtAllCooldown: 30 * time.Minute,
			QuietHours: GroupUXQuietHoursPolicy{
				Enabled:       false,
				Start:         "23:00",
				End:           "07:00",
				SummaryOnExit: true,
				SummaryMaxN:   8,
			},
		},
	}
	override := NewGroupUXOverride()
	override.SetBool(OverrideEnabled, false)
	override.SetDuration(OverrideNotifyDedupeTTL, 20*time.Second)
	override.SetString(OverrideNotifyAggregateTypes, "news,live")

	merged := MergePolicy(base, override)
	if merged.Enabled {
		t.Fatalf("expected merged.Enabled=false")
	}
	if merged.Notify.DedupeTTL != 20*time.Second {
		t.Fatalf("unexpected dedupe ttl: %v", merged.Notify.DedupeTTL)
	}
	if len(merged.Notify.Aggregate.Types) != 2 {
		t.Fatalf("unexpected aggregate types: %v", merged.Notify.Aggregate.Types)
	}
	if merged.Command.ParseErrorCooldown != 20*time.Second {
		t.Fatalf("parse error cooldown should keep base value")
	}
}

func TestInQuietHoursCrossMidnight(t *testing.T) {
	policy := GroupUXPolicy{
		Enabled: true,
		Notify: GroupUXNotifyPolicy{
			QuietHours: GroupUXQuietHoursPolicy{
				Enabled: true,
				Start:   "23:00",
				End:     "07:00",
			},
		},
	}
	loc := time.FixedZone("UTC+8", 8*3600)
	in1 := time.Date(2026, 2, 14, 23, 30, 0, 0, loc)
	in2 := time.Date(2026, 2, 15, 6, 59, 0, 0, loc)
	out := time.Date(2026, 2, 15, 8, 0, 0, 0, loc)

	if !InQuietHours(policy, in1) || !InQuietHours(policy, in2) {
		t.Fatalf("expected quiet hours to include 23:30 and 06:59")
	}
	if InQuietHours(policy, out) {
		t.Fatalf("expected quiet hours to exclude 08:00")
	}
}

func TestManagerDedupeTTL(t *testing.T) {
	m := NewManager(GroupUXPolicy{})
	policy := GroupUXPolicy{
		Enabled: true,
		Notify:  GroupUXNotifyPolicy{DedupeTTL: 2 * time.Second},
	}
	now := time.Now()
	if m.ShouldDropDuplicate(1, "bilibili", concern_type.Type("news"), "100", "hello", policy, now) {
		t.Fatalf("first notify should not be dropped")
	}
	if !m.ShouldDropDuplicate(1, "bilibili", concern_type.Type("news"), "100", "hello", policy, now.Add(500*time.Millisecond)) {
		t.Fatalf("second notify should be dropped in ttl")
	}
	if m.ShouldDropDuplicate(1, "bilibili", concern_type.Type("news"), "100", "hello", policy, now.Add(3*time.Second)) {
		t.Fatalf("notify after ttl should not be dropped")
	}
}

func TestManagerAggregateFlush(t *testing.T) {
	m := NewManager(GroupUXPolicy{})
	policy := GroupUXPolicy{
		Enabled: true,
		Notify: GroupUXNotifyPolicy{
			Aggregate: GroupUXAggregatePolicy{
				Enabled:  true,
				Window:   45 * time.Second,
				MaxItems: 2,
				Types:    []concern_type.Type{concern_type.Type("news")},
			},
		},
	}
	now := time.Now()
	item1 := QuietSummaryItem{Site: "bilibili", Type: concern_type.Type("news"), UID: "1", Message: "a", OccurredAt: now}
	item2 := QuietSummaryItem{Site: "bilibili", Type: concern_type.Type("news"), UID: "1", Message: "b", OccurredAt: now}

	if d := m.PushAggregate(100, concern_type.Type("news"), item1, policy, now); d != nil {
		t.Fatalf("first item should stay in bucket")
	}
	d := m.PushAggregate(100, concern_type.Type("news"), item2, policy, now)
	if d == nil || d.Count != 2 {
		t.Fatalf("second item should flush bucket, got=%+v", d)
	}
}

func TestManagerQuietHoursSummaryOnExit(t *testing.T) {
	m := NewManager(GroupUXPolicy{})
	policy := GroupUXPolicy{
		Enabled: true,
		Notify: GroupUXNotifyPolicy{
			QuietHours: GroupUXQuietHoursPolicy{
				Enabled:       true,
				Start:         "23:00",
				End:           "07:00",
				SummaryOnExit: true,
				SummaryMaxN:   8,
			},
		},
	}
	loc := time.FixedZone("UTC+8", 8*3600)
	quiet1 := time.Date(2026, 2, 14, 23, 10, 0, 0, loc)
	quiet2 := time.Date(2026, 2, 15, 6, 50, 0, 0, loc)
	normal := time.Date(2026, 2, 15, 7, 1, 0, 0, loc)

	item1 := QuietSummaryItem{Site: "a", Type: concern_type.Type("news"), UID: "1", Message: "one", OccurredAt: quiet1}
	item2 := QuietSummaryItem{Site: "a", Type: concern_type.Type("news"), UID: "2", Message: "two", OccurredAt: quiet2}
	item3 := QuietSummaryItem{Site: "a", Type: concern_type.Type("news"), UID: "3", Message: "three", OccurredAt: normal}

	deliver, summary := m.ApplyQuietHours(200, policy, item1, quiet1)
	if deliver || summary != nil {
		t.Fatalf("expected first quiet item to be held")
	}
	deliver, summary = m.ApplyQuietHours(200, policy, item2, quiet2)
	if deliver || summary != nil {
		t.Fatalf("expected second quiet item to be held")
	}
	deliver, summary = m.ApplyQuietHours(200, policy, item3, normal)
	if !deliver {
		t.Fatalf("expected non-quiet item to deliver")
	}
	if summary == nil || summary.Count != 2 {
		t.Fatalf("expected summary with 2 held items, got=%+v", summary)
	}
}

func TestStoreOverride(t *testing.T) {
	if err := localdb.InitBuntDB(localdb.MEMORYDB); err != nil {
		t.Fatalf("InitBuntDB failed: %v", err)
	}
	t.Cleanup(func() { _ = localdb.Close() })

	override := NewGroupUXOverride()
	override.SetBool(OverrideEnabled, false)
	override.SetDuration(OverrideNotifyDedupeTTL, 33*time.Second)
	if err := SaveOverride(999, override); err != nil {
		t.Fatalf("SaveOverride failed: %v", err)
	}
	loaded, err := LoadOverride(999)
	if err != nil {
		t.Fatalf("LoadOverride failed: %v", err)
	}
	if enabled, ok := loaded.GetBool(OverrideEnabled); !ok || enabled {
		t.Fatalf("loaded override enabled mismatch: %v, %v", enabled, ok)
	}
	if err := ResetOverride(999); err != nil {
		t.Fatalf("ResetOverride failed: %v", err)
	}
	loaded, err = LoadOverride(999)
	if err != nil {
		t.Fatalf("LoadOverride after reset failed: %v", err)
	}
	if !loaded.Empty() {
		t.Fatalf("expected empty override after reset")
	}
}
