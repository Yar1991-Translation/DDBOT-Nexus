package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type MigrationMappedField struct {
	OldKey      string
	NewKey      string
	ValueSource string
}

type MigrationUnknownField struct {
	Key         string
	ValueSource string
}

type MigrationDefaultField struct {
	OldKey string
	NewKey string
	Value  interface{}
}

type MigrationReport struct {
	InputPath        string
	OutputPath       string
	Mapped           []MigrationMappedField
	UnknownLegacy    []MigrationUnknownField
	DefaultsInjected []MigrationDefaultField
}

func MigrateConfigFile(inPath, outPath string) (MigrationReport, error) {
	if strings.TrimSpace(inPath) == "" {
		inPath = "application.yaml"
	}
	if strings.TrimSpace(outPath) == "" {
		outPath = "application.v2.yaml"
	}

	report := MigrationReport{
		InputPath: inPath,
	}

	legacyValues, err := parseYAMLFlat(inPath)
	if err != nil {
		return report, fmt.Errorf("parse legacy config %s: %w", inPath, err)
	}

	structuralKeys := buildStructuralKeySet(legacyValues)
	result := make(map[string]interface{})
	keys := sortedMapKeys(legacyValues)
	for _, rawKey := range keys {
		entry := legacyValues[rawKey]
		mapping, mapped := legacyMappingByLower[strings.ToLower(rawKey)]
		if mapped {
			setNestedValue(result, mapping.NewKey, entry.Value)
			report.Mapped = append(report.Mapped, MigrationMappedField{
				OldKey:      mapping.OldKey,
				NewKey:      mapping.NewKey,
				ValueSource: ValueSource{File: inPath, Line: entry.Line}.ValueSourceText(),
			})
			continue
		}
		if _, isStructural := structuralKeys[rawKey]; isStructural {
			continue
		}
		report.UnknownLegacy = append(report.UnknownLegacy, MigrationUnknownField{
			Key:         rawKey,
			ValueSource: ValueSource{File: inPath, Line: entry.Line}.ValueSourceText(),
		})
	}

	defaultKeys := sortedMapKeys(defaultsV1Keys)
	for _, oldKey := range defaultKeys {
		newKey, ok := oldToNewKeyMapping[oldKey]
		if !ok {
			continue
		}
		if hasNestedValue(result, newKey) {
			continue
		}
		setNestedValue(result, newKey, defaultsV1Keys[oldKey])
		report.DefaultsInjected = append(report.DefaultsInjected, MigrationDefaultField{
			OldKey: oldKey,
			NewKey: newKey,
			Value:  defaultsV1Keys[oldKey],
		})
	}

	data, err := marshalSortedYAML(result)
	if err != nil {
		return report, fmt.Errorf("marshal migrated yaml: %w", err)
	}

	outputPath, err := nextWritableMigrationPath(outPath)
	if err != nil {
		return report, err
	}
	if err := ensureParentDirectory(outputPath); err != nil {
		return report, err
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return report, fmt.Errorf("write migrated config %s: %w", outputPath, err)
	}
	report.OutputPath = outputPath
	return report, nil
}

func FormatMigrationReport(report MigrationReport) string {
	var b strings.Builder
	b.WriteString("config migration report\n")
	b.WriteString(fmt.Sprintf("input: %s\n", report.InputPath))
	b.WriteString(fmt.Sprintf("output: %s\n", report.OutputPath))
	b.WriteString(fmt.Sprintf("mapped_fields: %d\n", len(report.Mapped)))
	b.WriteString(fmt.Sprintf("unknown_legacy_fields: %d\n", len(report.UnknownLegacy)))
	b.WriteString(fmt.Sprintf("defaults_injected: %d\n", len(report.DefaultsInjected)))

	if len(report.Mapped) > 0 {
		b.WriteString("\n[mapped]\n")
		for _, item := range report.Mapped {
			b.WriteString(fmt.Sprintf("- %s -> %s (%s)\n", item.OldKey, item.NewKey, item.ValueSource))
		}
	}
	if len(report.UnknownLegacy) > 0 {
		b.WriteString("\n[unknown_legacy]\n")
		for _, item := range report.UnknownLegacy {
			b.WriteString(fmt.Sprintf("- %s (%s)\n", item.Key, item.ValueSource))
		}
	}
	if len(report.DefaultsInjected) > 0 {
		b.WriteString("\n[injected_defaults]\n")
		for _, item := range report.DefaultsInjected {
			b.WriteString(fmt.Sprintf("- %s -> %s = %v\n", item.OldKey, item.NewKey, item.Value))
		}
	}
	return b.String()
}

func buildStructuralKeySet(values map[string]flatYAMLValue) map[string]struct{} {
	result := make(map[string]struct{})
	for key := range values {
		parts := strings.Split(key, ".")
		for i := 1; i < len(parts); i++ {
			result[strings.Join(parts[:i], ".")] = struct{}{}
		}
	}
	return result
}

func setNestedValue(root map[string]interface{}, dottedKey string, value interface{}) {
	parts := strings.Split(dottedKey, ".")
	current := root
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = normalizeYAMLValue(value)
			return
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[part] = next
		}
		current = next
	}
}

func hasNestedValue(root map[string]interface{}, dottedKey string) bool {
	parts := strings.Split(dottedKey, ".")
	current := root
	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		next, ok := val.(map[string]interface{})
		if !ok {
			return false
		}
		current = next
	}
	return false
}

func marshalSortedYAML(data map[string]interface{}) ([]byte, error) {
	root, err := buildSortedYAMLNode(data)
	if err != nil {
		return nil, err
	}
	doc := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{root},
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildSortedYAMLNode(value interface{}) (*yaml.Node, error) {
	value = normalizeYAMLValue(value)
	switch typed := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, key := range keys {
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: key,
			})
			child, err := buildSortedYAMLNode(typed[key])
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		return node, nil
	case []interface{}:
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range typed {
			child, err := buildSortedYAMLNode(item)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		return node, nil
	default:
		var node yaml.Node
		if err := node.Encode(typed); err != nil {
			return nil, err
		}
		return &node, nil
	}
}

func nextWritableMigrationPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("output path is a directory: %s", path)
		}
		return nextAvailableMigrationPath(path), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

func nextAvailableMigrationPath(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = "application.v2"
	}
	candidate := filepath.Join(dir, stem+".migrated"+ext)
	index := 2
	for pathExists(candidate) {
		candidate = filepath.Join(dir, fmt.Sprintf("%s.migrated.%d%s", stem, index, ext))
		index++
	}
	return candidate
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ensureParentDirectory(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
