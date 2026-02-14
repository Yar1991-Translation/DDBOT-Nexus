package groupux

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
)

type quietBuffer struct {
	inQuiet bool
	items   []QuietSummaryItem
	dropped int
}

type Manager struct {
	mu sync.Mutex

	global GroupUXPolicy

	overrides      map[int64]GroupUXOverride
	overrideLoaded map[int64]bool

	dedupe   map[string]time.Time
	atAll    map[int64]time.Time
	parseErr map[string]time.Time

	aggregate map[string]*DigestBucket
	quiet     map[int64]*quietBuffer
}

func NewManager(global GroupUXPolicy) *Manager {
	return &Manager{
		global:         sanitizePolicy(global),
		overrides:      make(map[int64]GroupUXOverride),
		overrideLoaded: make(map[int64]bool),
		dedupe:         make(map[string]time.Time),
		atAll:          make(map[int64]time.Time),
		parseErr:       make(map[string]time.Time),
		aggregate:      make(map[string]*DigestBucket),
		quiet:          make(map[int64]*quietBuffer),
	}
}

func (m *Manager) SetGlobalPolicy(policy GroupUXPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.global = sanitizePolicy(policy)
}

func (m *Manager) GetGlobalPolicy() GroupUXPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.global.Clone()
}

func (m *Manager) GetGroupOverride(groupCode int64) (GroupUXOverride, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getOverrideLocked(groupCode)
}

func (m *Manager) SaveGroupOverride(groupCode int64, override GroupUXOverride) error {
	override = override.Clone()
	override.normalize()
	if err := SaveOverride(groupCode, override); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if override.Empty() {
		delete(m.overrides, groupCode)
	} else {
		m.overrides[groupCode] = override
	}
	m.overrideLoaded[groupCode] = true
	return nil
}

func (m *Manager) ResetGroupOverride(groupCode int64) error {
	if err := ResetOverride(groupCode); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.overrides, groupCode)
	m.overrideLoaded[groupCode] = true
	return nil
}

func (m *Manager) Policy(groupCode int64) GroupUXPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	override, err := m.getOverrideLocked(groupCode)
	if err != nil {
		return m.global.Clone()
	}
	return MergePolicy(m.global, override)
}

func (m *Manager) getOverrideLocked(groupCode int64) (GroupUXOverride, error) {
	if loaded := m.overrideLoaded[groupCode]; loaded {
		if override, ok := m.overrides[groupCode]; ok {
			return override.Clone(), nil
		}
		return NewGroupUXOverride(), nil
	}
	override, err := LoadOverride(groupCode)
	if err != nil {
		return NewGroupUXOverride(), err
	}
	if override.Empty() {
		delete(m.overrides, groupCode)
	} else {
		m.overrides[groupCode] = override.Clone()
	}
	m.overrideLoaded[groupCode] = true
	return override, nil
}

func (m *Manager) AllowParseError(groupCode, userID int64, command string, policy GroupUXPolicy, now time.Time) bool {
	if !policy.Enabled || policy.Command.ParseErrorCooldown <= 0 {
		return true
	}
	key := fmt.Sprintf("%d:%d:%s", groupCode, userID, strings.ToLower(strings.TrimSpace(command)))
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.parseErr[key]; ok && exp.After(now) {
		return false
	}
	m.parseErr[key] = now.Add(policy.Command.ParseErrorCooldown)
	if len(m.parseErr) > 4096 {
		for k, exp := range m.parseErr {
			if !exp.After(now) {
				delete(m.parseErr, k)
			}
		}
	}
	return true
}

func (m *Manager) AllowAtAll(groupCode int64, policy GroupUXPolicy, now time.Time) bool {
	if !policy.Enabled || policy.Notify.AtAllCooldown <= 0 {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if last, ok := m.atAll[groupCode]; ok {
		if now.Sub(last) < policy.Notify.AtAllCooldown {
			return false
		}
	}
	m.atAll[groupCode] = now
	return true
}

func (m *Manager) ShouldDropDuplicate(groupCode int64, site string, ctype concern_type.Type, uid string, message string, policy GroupUXPolicy, now time.Time) bool {
	if !policy.Enabled || policy.Notify.DedupeTTL <= 0 {
		return false
	}
	fingerprint := notificationFingerprint(groupCode, site, ctype, uid, message)
	expiresAt := now.Add(policy.Notify.DedupeTTL)
	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.dedupe[fingerprint]; ok && prev.After(now) {
		return true
	}
	m.dedupe[fingerprint] = expiresAt
	if len(m.dedupe) > 4096 {
		for key, exp := range m.dedupe {
			if !exp.After(now) {
				delete(m.dedupe, key)
			}
		}
	}
	return false
}

func (m *Manager) GroupUnknownTipsEnabled(groupCode int64) bool {
	policy := m.Policy(groupCode)
	return policy.Enabled && policy.Command.GroupUnknownTips
}

func (m *Manager) FlushDue(now time.Time) []PreparedDigest {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]PreparedDigest, 0)

	for key, bucket := range m.aggregate {
		if bucket == nil || now.Before(bucket.FlushAt) {
			continue
		}
		policy, err := m.policyLocked(bucket.GroupCode)
		if err != nil {
			policy = m.global
		}
		prepared := PreparedDigest{
			GroupCode: bucket.GroupCode,
			Type:      bucket.Type,
			Items:     append([]QuietSummaryItem{}, bucket.Items...),
			Count:     len(bucket.Items),
			Policy:    policy,
			Reason:    "aggregate",
		}
		result = append(result, prepared)
		delete(m.aggregate, key)
	}

	for groupCode, quiet := range m.quiet {
		if quiet == nil {
			continue
		}
		policy, err := m.policyLocked(groupCode)
		if err != nil {
			policy = m.global
		}
		nowQuiet := InQuietHours(policy, now)
		if quiet.inQuiet && !nowQuiet {
			if len(quiet.items) > 0 && policy.Notify.QuietHours.SummaryOnExit {
				result = append(result, PreparedDigest{
					GroupCode: groupCode,
					Type:      concern_type.Type("digest"),
					Items:     append([]QuietSummaryItem{}, quiet.items...),
					Count:     len(quiet.items),
					Dropped:   quiet.dropped,
					Policy:    policy,
					Reason:    "quiet-summary",
				})
			}
			quiet.items = nil
			quiet.dropped = 0
			quiet.inQuiet = false
			continue
		}
		quiet.inQuiet = nowQuiet
	}

	return result
}

func (m *Manager) PushAggregate(groupCode int64, ctype concern_type.Type, item QuietSummaryItem, policy GroupUXPolicy, now time.Time) *PreparedDigest {
	if !AggregateTypeEnabled(policy, ctype) {
		return nil
	}
	if policy.Notify.Aggregate.Window <= 0 || policy.Notify.Aggregate.MaxItems <= 1 {
		return &PreparedDigest{
			GroupCode: groupCode,
			Type:      ctype,
			Items:     []QuietSummaryItem{item},
			Count:     1,
			Policy:    policy,
			Reason:    "aggregate",
		}
	}

	key := fmt.Sprintf("%d:%s", groupCode, strings.ToLower(ctype.String()))

	m.mu.Lock()
	defer m.mu.Unlock()
	bucket := m.aggregate[key]
	if bucket == nil {
		bucket = &DigestBucket{
			GroupCode: groupCode,
			Type:      ctype,
			FlushAt:   now.Add(policy.Notify.Aggregate.Window),
		}
		m.aggregate[key] = bucket
	}
	bucket.Items = append(bucket.Items, item)
	if len(bucket.Items) < policy.Notify.Aggregate.MaxItems {
		return nil
	}
	prepared := &PreparedDigest{
		GroupCode: groupCode,
		Type:      ctype,
		Items:     append([]QuietSummaryItem{}, bucket.Items...),
		Count:     len(bucket.Items),
		Policy:    policy,
		Reason:    "aggregate",
	}
	delete(m.aggregate, key)
	return prepared
}

func (m *Manager) ApplyQuietHours(groupCode int64, policy GroupUXPolicy, item QuietSummaryItem, now time.Time) (bool, *PreparedDigest) {
	if !policy.Enabled || !policy.Notify.QuietHours.Enabled {
		return true, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	quiet := m.quiet[groupCode]
	if quiet == nil {
		quiet = &quietBuffer{}
		m.quiet[groupCode] = quiet
	}
	nowQuiet := InQuietHours(policy, now)

	var summary *PreparedDigest
	if quiet.inQuiet && !nowQuiet {
		if len(quiet.items) > 0 && policy.Notify.QuietHours.SummaryOnExit {
			summary = &PreparedDigest{
				GroupCode: groupCode,
				Type:      concern_type.Type("digest"),
				Items:     append([]QuietSummaryItem{}, quiet.items...),
				Count:     len(quiet.items),
				Dropped:   quiet.dropped,
				Policy:    policy,
				Reason:    "quiet-summary",
			}
		}
		quiet.items = nil
		quiet.dropped = 0
	}

	quiet.inQuiet = nowQuiet
	if !nowQuiet {
		return true, summary
	}

	if len(quiet.items) >= 500 {
		quiet.dropped++
		return false, summary
	}
	quiet.items = append(quiet.items, item)
	return false, summary
}

func (m *Manager) policyLocked(groupCode int64) (GroupUXPolicy, error) {
	override, err := m.getOverrideLocked(groupCode)
	if err != nil {
		return m.global.Clone(), err
	}
	return MergePolicy(m.global, override), nil
}

func notificationFingerprint(groupCode int64, site string, ctype concern_type.Type, uid string, message string) string {
	raw := fmt.Sprintf("%d|%s|%s|%s|%s", groupCode, strings.ToLower(strings.TrimSpace(site)), strings.ToLower(strings.TrimSpace(ctype.String())), strings.TrimSpace(uid), strings.TrimSpace(message))
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}
