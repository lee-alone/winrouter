package observability

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var secretKey = regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|authorization|credential|subscription|uuid)`)
var uuidValue = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)
var inlineSecret = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|authorization|credential|subscription|uuid)\s*[:=]\s*[^\s&;,]+`)

func Preview(input BundleInput) BundlePreview {
	return BundlePreview{Files: []string{"manifest.json", "application.json", "interfaces.json", "core.json", "logs.json", "probes.json", "interface-counters.json", "rule-sets.json", "dns-status.json"}, LogCount: len(input.Logs), ProbeCount: len(input.Probes), RuleSetCount: len(input.RuleSets), SensitiveNotes: []string{"密码、令牌、UUID、订阅凭据和 API secret 替换为 [REDACTED]", "URL 敏感查询参数被移除", "不包含完整 rule-set 二进制"}}
}

func WriteBundle(path string, input BundleInput) error {
	if err := ValidateBundlePath(path); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".winrouter-diagnostic-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	ok := false
	defer func() {
		file.Close()
		if !ok {
			_ = os.Remove(temporary)
		}
	}()
	archive := zip.NewWriter(file)
	items := []struct {
		name  string
		value any
	}{{"manifest.json", Preview(input)}, {"application.json", input.Application}, {"interfaces.json", input.Interfaces}, {"core.json", input.Core}, {"logs.json", input.Logs}, {"probes.json", input.Probes}, {"interface-counters.json", input.Counters}, {"rule-sets.json", input.RuleSets}, {"dns-status.json", input.DNS}}
	for _, item := range items {
		if err = writeJSON(archive, item.name, item.value); err != nil {
			archive.Close()
			return err
		}
	}
	if err = archive.Close(); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	_ = os.Remove(path)
	if err = os.Rename(temporary, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func writeJSON(archive *zip.Writer, name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var decoded any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	redacted := redact(decoded, "")
	data, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return err
	}
	entry, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, bytes.NewReader(append(data, '\n')))
	return err
}

func redact(value any, key string) any {
	if secretKey.MatchString(key) {
		return "[REDACTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		for k, v := range typed {
			typed[k] = redact(v, k)
		}
		return typed
	case []any:
		for i, v := range typed {
			typed[i] = redact(v, key)
		}
		return typed
	case string:
		return redactString(typed)
	default:
		return value
	}
}

func redactString(value string) string {
	parsed, err := url.Parse(value)
	if err == nil && parsed.IsAbs() && parsed.RawQuery != "" {
		query := parsed.Query()
		for key := range query {
			if secretKey.MatchString(key) {
				query.Set(key, "[REDACTED]")
			}
		}
		parsed.RawQuery = query.Encode()
		value = parsed.String()
	}
	if strings.Contains(strings.ToLower(value), "bearer ") {
		return "[REDACTED]"
	}
	value = uuidValue.ReplaceAllString(value, "[REDACTED]")
	return inlineSecret.ReplaceAllString(value, "$1=[REDACTED]")
}

func ValidateBundlePath(path string) error {
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".zip") {
		return fmt.Errorf("diagnostic bundle path must end in .zip")
	}
	return nil
}
