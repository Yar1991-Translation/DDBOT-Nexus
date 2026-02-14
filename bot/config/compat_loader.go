package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type flatYAMLValue struct {
	Value interface{}
	Line  int
}

type legacyMapping struct {
	OldKey string
	NewKey string
}

var (
	legacyMappingByLower       = buildLegacyMappingByLower()
	v2MappingByLower           = buildV2MappingByLower()
	knownV2StructuralPrefixes  = buildKnownV2StructuralPrefixes()
	v2OnlyPassthroughByLower   = buildV2OnlyPassthroughByLower()
	deprecatedWarned           sync.Map
	deprecatedWarnedTotalCount atomic.Int64
)

func loadCanonicalByPath(searchPath []string, configName, configType string) (ConfigCanonical, Diagnostics, resolvedConfigPaths, error) {
	resolved := resolveConfigPaths(searchPath, configName, configType)
	if resolved.LegacyPath == "" && resolved.V2Path == "" {
		return ConfigCanonical{}, Diagnostics{}, resolved, fmt.Errorf("no config file found: %s or %s", resolved.legacyFileName(), resolved.v2FileName())
	}

	canonical := defaultCanonical()
	diagnostics := Diagnostics{}

	if resolved.LegacyPath != "" {
		legacyEntries, err := parseYAMLFlat(resolved.LegacyPath)
		if err != nil {
			return ConfigCanonical{}, Diagnostics{}, resolved, fmt.Errorf("parse legacy config %s: %w", resolved.LegacyPath, err)
		}
		canonical = MergeCanonical(canonical, legacyToCanonical(legacyEntries, resolved.LegacyPath, &diagnostics))
	}
	if resolved.V2Path != "" {
		v2Entries, err := parseYAMLFlat(resolved.V2Path)
		if err != nil {
			return ConfigCanonical{}, Diagnostics{}, resolved, fmt.Errorf("parse v2 config %s: %w", resolved.V2Path, err)
		}
		canonical = MergeCanonical(canonical, v2ToCanonical(v2Entries, resolved.V2Path, &diagnostics))
	}
	return canonical, diagnostics, resolved, nil
}

func resolveConfigPaths(searchPath []string, configName, configType string) resolvedConfigPaths {
	if len(searchPath) == 0 {
		searchPath = []string{"."}
	}
	resolved := resolvedConfigPaths{
		Name:       configName,
		Type:       configType,
		SearchPath: append([]string{}, searchPath...),
	}
	legacyFileName := resolved.legacyFileName()
	v2FileName := resolved.v2FileName()
	for _, root := range resolved.SearchPath {
		if root == "" {
			continue
		}
		if resolved.LegacyPath == "" {
			candidate := filepath.Clean(filepath.Join(root, legacyFileName))
			if isRegularFile(candidate) {
				resolved.LegacyPath = candidate
			}
		}
		if resolved.V2Path == "" {
			candidate := filepath.Clean(filepath.Join(root, v2FileName))
			if isRegularFile(candidate) {
				resolved.V2Path = candidate
			}
		}
		if resolved.LegacyPath != "" && resolved.V2Path != "" {
			break
		}
	}
	return resolved
}

func parseYAMLFlat(file string) (map[string]flatYAMLValue, error) {
	result := make(map[string]flatYAMLValue)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return result, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return result, nil
	}
	if root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("yaml root must be mapping")
	}
	if err := flattenYAML(root.Content[0], "", 0, result); err != nil {
		return nil, err
	}
	return result, nil
}

func flattenYAML(node *yaml.Node, keyPath string, line int, out map[string]flatYAMLValue) error {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil
		}
		return flattenYAML(node.Content[0], keyPath, line, out)
	case yaml.MappingNode:
		if keyPath != "" {
			value, err := decodeNodeValue(node)
			if err != nil {
				return err
			}
			out[keyPath] = flatYAMLValue{Value: value, Line: line}
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			childKey := strings.TrimSpace(keyNode.Value)
			if childKey == "" {
				continue
			}
			if keyPath != "" {
				childKey = keyPath + "." + childKey
			}
			if err := flattenYAML(valueNode, childKey, keyNode.Line, out); err != nil {
				return err
			}
		}
		return nil
	default:
		if keyPath == "" {
			return fmt.Errorf("yaml root must be mapping")
		}
		value, err := decodeNodeValue(node)
		if err != nil {
			return err
		}
		out[keyPath] = flatYAMLValue{Value: value, Line: line}
		return nil
	}
}

func decodeNodeValue(node *yaml.Node) (interface{}, error) {
	var value interface{}
	if err := node.Decode(&value); err != nil {
		return nil, err
	}
	return normalizeYAMLValue(value), nil
}

func normalizeYAMLValue(in interface{}) interface{} {
	switch typed := in.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(typed))
		for key, val := range typed {
			m[key] = normalizeYAMLValue(val)
		}
		return m
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(typed))
		for key, val := range typed {
			m[fmt.Sprint(key)] = normalizeYAMLValue(val)
		}
		return m
	case []interface{}:
		arr := make([]interface{}, 0, len(typed))
		for _, val := range typed {
			arr = append(arr, normalizeYAMLValue(val))
		}
		return arr
	default:
		return in
	}
}

func defaultCanonical() ConfigCanonical {
	result := newCanonical()
	keys := sortedMapKeys(defaultsV1Keys)
	for _, key := range keys {
		result.Set(key, defaultsV1Keys[key], ValueSource{
			Layer: "default",
			File:  "default",
			Key:   key,
		}, true)
	}
	return result
}

func legacyToCanonical(values map[string]flatYAMLValue, fromFile string, diagnostics *Diagnostics) ConfigCanonical {
	result := newCanonical()
	keys := sortedMapKeys(values)
	for _, rawKey := range keys {
		entry := values[rawKey]
		mapped, mappedOk := legacyMappingByLower[strings.ToLower(rawKey)]
		canonicalKey := rawKey
		if mappedOk {
			canonicalKey = mapped.OldKey
		}
		src := ValueSource{
			Layer: "old",
			File:  fromFile,
			Line:  entry.Line,
			Key:   rawKey,
		}
		result.Set(canonicalKey, entry.Value, src, true)
		if mappedOk && mapped.OldKey != mapped.NewKey {
			diagnostics.addDeprecated(mapped.OldKey, mapped.NewKey, src)
		}
	}
	return result
}

func v2ToCanonical(values map[string]flatYAMLValue, fromFile string, diagnostics *Diagnostics) ConfigCanonical {
	result := newCanonical()
	keys := sortedMapKeys(values)
	for _, rawKey := range keys {
		entry := values[rawKey]
		keyLower := strings.ToLower(rawKey)
		if mappedOldKey, ok := v2MappingByLower[keyLower]; ok {
			result.Set(mappedOldKey, entry.Value, ValueSource{
				Layer: "new",
				File:  fromFile,
				Line:  entry.Line,
				Key:   rawKey,
			}, true)
			continue
		}
		if isV2OnlyPassthroughKey(keyLower) {
			result.Set(rawKey, entry.Value, ValueSource{
				Layer: "new",
				File:  fromFile,
				Line:  entry.Line,
				Key:   rawKey,
			}, true)
			continue
		}
		if _, structural := knownV2StructuralPrefixes[keyLower]; structural {
			continue
		}
		if hasMappedAncestor(keyLower) {
			continue
		}
		diagnostics.addUnknownV2(rawKey, ValueSource{
			Layer: "new",
			File:  fromFile,
			Line:  entry.Line,
			Key:   rawKey,
		})
	}
	return result
}

func buildLegacyMappingByLower() map[string]legacyMapping {
	result := make(map[string]legacyMapping, len(oldToNewKeyMapping))
	for oldKey, newKey := range oldToNewKeyMapping {
		lower := strings.ToLower(oldKey)
		if _, exists := result[lower]; exists {
			continue
		}
		result[lower] = legacyMapping{
			OldKey: oldKey,
			NewKey: newKey,
		}
	}
	return result
}

func buildV2MappingByLower() map[string]string {
	result := make(map[string]string, len(newToOldKeyMapping))
	for newKey, oldKey := range newToOldKeyMapping {
		lower := strings.ToLower(newKey)
		if _, exists := result[lower]; exists {
			continue
		}
		result[lower] = oldKey
	}
	return result
}

func buildKnownV2StructuralPrefixes() map[string]struct{} {
	result := make(map[string]struct{})
	for _, newKey := range oldToNewKeyMapping {
		parts := strings.Split(strings.ToLower(newKey), ".")
		for i := 1; i < len(parts); i++ {
			prefix := strings.Join(parts[:i], ".")
			result[prefix] = struct{}{}
		}
	}
	return result
}

func buildV2OnlyPassthroughByLower() map[string]struct{} {
	result := make(map[string]struct{}, len(v2OnlyPassthroughPrefixes))
	for _, prefix := range v2OnlyPassthroughPrefixes {
		key := strings.TrimSpace(strings.ToLower(prefix))
		if key == "" {
			continue
		}
		result[key] = struct{}{}
	}
	return result
}

func isV2OnlyPassthroughKey(keyLower string) bool {
	for prefix := range v2OnlyPassthroughByLower {
		if keyLower == prefix || strings.HasPrefix(keyLower, prefix+".") {
			return true
		}
	}
	return false
}

func hasMappedAncestor(keyLower string) bool {
	parts := strings.Split(keyLower, ".")
	for i := len(parts) - 1; i > 0; i-- {
		if _, ok := v2MappingByLower[strings.Join(parts[:i], ".")]; ok {
			return true
		}
	}
	return false
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func sortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func logDiagnostics(diagnostics Diagnostics) {
	newDeprecatedCount := 0
	for _, warning := range diagnostics.Deprecated {
		if _, loaded := deprecatedWarned.LoadOrStore(warning.OldKey, struct{}{}); loaded {
			continue
		}
		logrus.Warnf("[DEPRECATED_CONFIG] %s -> %s (value_source=%s)", warning.OldKey, warning.NewKey, warning.ValueSource)
		deprecatedWarnedTotalCount.Add(1)
		newDeprecatedCount++
	}
	if newDeprecatedCount > 0 {
		logrus.Warnf("[DEPRECATED_CONFIG] total=%d", deprecatedWarnedTotalCount.Load())
	}
	for _, warning := range diagnostics.UnknownV2 {
		logrus.Warnf("[UNKNOWN_V2_CONFIG] %s (value_source=%s)", warning.Key, warning.ValueSource)
	}
}
