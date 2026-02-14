package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

func TestLoadCanonicalOldOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.yaml"), `
websocket:
  mode: ws-server
notify:
  parallel: 2
`)

	canonical, diagnostics, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}

	if got := canonical.Values["websocket.mode"]; got != "ws-server" {
		t.Fatalf("websocket.mode mismatch, got=%v", got)
	}
	if got := canonical.Values["notify.parallel"]; got != 2 {
		t.Fatalf("notify.parallel mismatch, got=%v", got)
	}
	if got := canonical.Values["dispatch.largeNotifyLimit"]; got != 50 {
		t.Fatalf("dispatch.largeNotifyLimit default mismatch, got=%v", got)
	}
	if canonical.Sources["websocket.mode"].Layer != "old" {
		t.Fatalf("websocket.mode source should be old")
	}
	if len(diagnostics.Deprecated) == 0 {
		t.Fatalf("expected deprecated warnings for legacy keys")
	}
}

func TestLoadCanonicalV2Only(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.v2.yaml"), `
transport:
  websocket:
    mode: ws-reverse
    wsServer: 127.0.0.1:15630
notify:
  dispatch:
    largeNotifyLimit: 88
`)

	canonical, diagnostics, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}

	if got := canonical.Values["websocket.mode"]; got != "ws-reverse" {
		t.Fatalf("websocket.mode mismatch, got=%v", got)
	}
	if got := canonical.Values["websocket.ws-server"]; got != "127.0.0.1:15630" {
		t.Fatalf("websocket.ws-server mismatch, got=%v", got)
	}
	if got := canonical.Values["dispatch.largeNotifyLimit"]; got != 88 {
		t.Fatalf("dispatch.largeNotifyLimit mismatch, got=%v", got)
	}
	if canonical.Sources["websocket.mode"].Layer != "new" {
		t.Fatalf("websocket.mode source should be new")
	}
	if len(diagnostics.Deprecated) != 0 {
		t.Fatalf("deprecated warnings should be empty for v2-only config")
	}
	if len(diagnostics.UnknownV2) != 0 {
		t.Fatalf("unknown v2 fields should be empty, got=%v", diagnostics.UnknownV2)
	}
}

func TestLoadCanonicalV2RobloxPassThrough(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.v2.yaml"), `
providers:
  roblox:
    enabled: true
    interval: 45s
    timeout: 9s
    batchSize: 80
    userAgent: test-agent
`)

	canonical, diagnostics, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}

	if got := canonical.Values["providers.roblox.enabled"]; got != true {
		t.Fatalf("providers.roblox.enabled mismatch, got=%v", got)
	}
	if got := canonical.Values["providers.roblox.interval"]; got != "45s" {
		t.Fatalf("providers.roblox.interval mismatch, got=%v", got)
	}
	if got := canonical.Values["providers.roblox.timeout"]; got != "9s" {
		t.Fatalf("providers.roblox.timeout mismatch, got=%v", got)
	}
	if got := canonical.Values["providers.roblox.batchSize"]; got != 80 {
		t.Fatalf("providers.roblox.batchSize mismatch, got=%v", got)
	}
	if canonical.Sources["providers.roblox.enabled"].Layer != "new" {
		t.Fatalf("providers.roblox.enabled source should be new")
	}
	if len(diagnostics.UnknownV2) != 0 {
		t.Fatalf("roblox v2 passthrough keys should not be unknown, got=%v", diagnostics.UnknownV2)
	}
}

func TestLoadCanonicalGroupUXOldOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.yaml"), `
groupUX:
  enabled: false
  command:
    parseErrorCooldown: 12s
  notify:
    aggregate:
      enabled: false
      types:
        - news
        - live
`)

	canonical, diagnostics, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}
	if got := canonical.Values["groupUX.enabled"]; got != false {
		t.Fatalf("groupUX.enabled mismatch, got=%v", got)
	}
	if got := canonical.Values["groupUX.command.parseErrorCooldown"]; got != "12s" {
		t.Fatalf("groupUX.command.parseErrorCooldown mismatch, got=%v", got)
	}
	if got := canonical.Values["groupUX.notify.aggregate.enabled"]; got != false {
		t.Fatalf("groupUX.notify.aggregate.enabled mismatch, got=%v", got)
	}
	if got := canonical.Values["groupUX.notify.aggregate.types"]; len(got.([]interface{})) != 2 {
		t.Fatalf("groupUX.notify.aggregate.types mismatch, got=%v", got)
	}
	if len(diagnostics.Deprecated) == 0 {
		t.Fatalf("expected deprecated warnings for groupUX legacy keys")
	}
}

func TestLoadCanonicalGroupUXV2Only(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.v2.yaml"), `
features:
  groupUx:
    enabled: false
    command:
      groupUnknownTips: true
notify:
  groupUx:
    dedupeTTL: 80s
    aggregate:
      window: 30s
`)

	canonical, diagnostics, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}
	if got := canonical.Values["groupUX.enabled"]; got != false {
		t.Fatalf("groupUX.enabled mismatch, got=%v", got)
	}
	if got := canonical.Values["groupUX.command.groupUnknownTips"]; got != true {
		t.Fatalf("groupUX.command.groupUnknownTips mismatch, got=%v", got)
	}
	if got := canonical.Values["groupUX.notify.dedupeTTL"]; got != "80s" {
		t.Fatalf("groupUX.notify.dedupeTTL mismatch, got=%v", got)
	}
	if got := canonical.Values["groupUX.notify.aggregate.window"]; got != "30s" {
		t.Fatalf("groupUX.notify.aggregate.window mismatch, got=%v", got)
	}
	if len(diagnostics.UnknownV2) != 0 {
		t.Fatalf("groupUx v2 keys should not be unknown, got=%v", diagnostics.UnknownV2)
	}
}

func TestLoadCanonicalGroupUXBothV2Wins(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.yaml"), `
groupUX:
  notify:
    dedupeTTL: 120s
`)
	writeTestFile(t, filepath.Join(dir, "application.v2.yaml"), `
notify:
  groupUx:
    dedupeTTL: 45s
`)

	canonical, _, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}
	if got := canonical.Values["groupUX.notify.dedupeTTL"]; got != "45s" {
		t.Fatalf("v2 key should win conflicts, got=%v", got)
	}
	if canonical.Sources["groupUX.notify.dedupeTTL"].Layer != "new" {
		t.Fatalf("groupUX.notify.dedupeTTL source should be new")
	}
}

func TestLoadCanonicalGroupUXUnknownV2(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.v2.yaml"), `
notify:
  groupUx:
    unknownField: 1
`)

	_, diagnostics, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}
	if len(diagnostics.UnknownV2) == 0 {
		t.Fatalf("expected unknown v2 field for notify.groupUx.unknownField")
	}
}

func TestLoadCanonicalBothConflictV2Wins(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.yaml"), `
websocket:
  mode: ws-server
notify:
  parallel: 1
`)
	writeTestFile(t, filepath.Join(dir, "application.v2.yaml"), `
transport:
  websocket:
    mode: ws-reverse
`)

	canonical, _, _, err := loadCanonicalByPath([]string{dir}, "application", "yaml")
	if err != nil {
		t.Fatalf("loadCanonicalByPath failed: %v", err)
	}

	if got := canonical.Values["websocket.mode"]; got != "ws-reverse" {
		t.Fatalf("v2 should win conflicts, got=%v", got)
	}
	if canonical.Sources["websocket.mode"].Layer != "new" {
		t.Fatalf("websocket.mode source should be new")
	}
	if got := canonical.Values["notify.parallel"]; got != 1 {
		t.Fatalf("legacy fallback expected, got=%v", got)
	}
}

func TestLogDiagnosticsDeprecatedOncePerStartup(t *testing.T) {
	deprecatedWarned = sync.Map{}
	deprecatedWarnedTotalCount.Store(0)

	var buf bytes.Buffer
	oldOutput := logrus.StandardLogger().Out
	oldFormatter := logrus.StandardLogger().Formatter
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableSorting:   true,
	})
	t.Cleanup(func() {
		logrus.SetOutput(oldOutput)
		logrus.SetFormatter(oldFormatter)
		deprecatedWarned = sync.Map{}
		deprecatedWarnedTotalCount.Store(0)
	})

	diagnostics := Diagnostics{}
	diagnostics.addDeprecated("websocket.ws-server", "transport.websocket.wsServer", ValueSource{
		File: "application.yaml",
		Line: 12,
	})

	logDiagnostics(diagnostics)
	logDiagnostics(diagnostics)

	output := buf.String()
	warnLine := "[DEPRECATED_CONFIG] websocket.ws-server -> transport.websocket.wsServer (value_source=application.yaml:12)"
	if strings.Count(output, warnLine) != 1 {
		t.Fatalf("expected exactly one deprecated warning line, output=%s", output)
	}
	if strings.Count(output, "[DEPRECATED_CONFIG] total=") != 1 {
		t.Fatalf("expected exactly one deprecated summary line, output=%s", output)
	}
}

func TestWatchConfigDualFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application.yaml"), `
websocket:
  mode: ws-server
`)

	cfg := NewConfig()
	cfg.SetConfigName("application")
	cfg.SetConfigType("yaml")
	cfg.AddConfigPath(dir)
	if err := cfg.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig failed: %v", err)
	}
	if got := cfg.GetString("websocket.mode"); got != "ws-server" {
		t.Fatalf("initial websocket.mode mismatch, got=%s", got)
	}

	events := make(chan struct{}, 8)
	cfg.OnConfigChange(func(in fsnotify.Event) {
		events <- struct{}{}
	})
	cfg.WatchConfig()
	defer cfg.CloseWatcher()
	time.Sleep(200 * time.Millisecond)

	writeTestFile(t, filepath.Join(dir, "application.v2.yaml"), `
transport:
  websocket:
    mode: ws-reverse
`)
	waitForEvent(t, events, "write v2")
	waitForValue(t, 3*time.Second, func() bool {
		return cfg.GetString("websocket.mode") == "ws-reverse"
	}, "websocket.mode should switch to ws-reverse after v2 update")

	writeTestFile(t, filepath.Join(dir, "application.yaml"), `
websocket:
  mode: ws-server-modified
`)
	waitForEvent(t, events, "write legacy")
	waitForValue(t, 3*time.Second, func() bool {
		return cfg.GetString("websocket.mode") == "ws-reverse"
	}, "v2 should still win after legacy update")
}

func TestMigrateConfigFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "application.yaml")
	out := filepath.Join(dir, "application.v2.yaml")
	writeTestFile(t, in, `
websocket:
  mode: ws-server
notify:
  parallel: 3
legacyOnly:
  enabled: true
`)

	report, err := MigrateConfigFile(in, out)
	if err != nil {
		t.Fatalf("MigrateConfigFile failed: %v", err)
	}
	if report.OutputPath != out {
		t.Fatalf("unexpected output path: %s", report.OutputPath)
	}
	if len(report.Mapped) == 0 {
		t.Fatalf("expected mapped fields")
	}
	if len(report.UnknownLegacy) == 0 {
		t.Fatalf("expected unknown legacy fields")
	}
	if len(report.DefaultsInjected) == 0 {
		t.Fatalf("expected default injections")
	}

	data, err := os.ReadFile(report.OutputPath)
	if err != nil {
		t.Fatalf("read migrated output failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "transport:") || !strings.Contains(content, "websocket:") {
		t.Fatalf("migrated output missing transport.websocket section:\n%s", content)
	}

	report2, err := MigrateConfigFile(in, out)
	if err != nil {
		t.Fatalf("second migrate run failed: %v", err)
	}
	if report2.OutputPath == out {
		t.Fatalf("second migrate should not overwrite existing output")
	}
	if _, err := os.Stat(report2.OutputPath); err != nil {
		t.Fatalf("expected secondary output file to exist: %v", err)
	}
}

func writeTestFile(t *testing.T, file string, content string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write test file failed: %v", err)
	}
}

func waitForEvent(t *testing.T, ch <-chan struct{}, reason string) {
	t.Helper()
	select {
	case <-ch:
		return
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for config event: %s", reason)
	}
}

func waitForValue(t *testing.T, timeout time.Duration, cond func() bool, failMsg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf(failMsg)
}
