package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	deepSeekHarnessProviderID    = "zcode-bridge"
	deepSeekHarnessProviderName  = "ZCode Local Bridge"
	deepSeekHarnessCredentialRef = "ZCODE_BRIDGE_API_KEY"
)

type deepSeekHarnessInstallResult struct {
	Home              string
	SettingsPath      string
	CredentialsPath   string
	SettingsBackup    string
	CredentialsBackup string
}

func (a *app) configureDeepSeekHarness(baseURL, model string, imageInput bool) (deepSeekHarnessInstallResult, error) {
	var result deepSeekHarnessInstallResult
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/v1"
	model = strings.TrimSpace(model)
	if model == "" {
		return result, errors.New("DeepSeek Harness 接入失败：没有可用模型")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "http" || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
		return result, errors.New("DeepSeek Harness 只允许接入本机回环网关")
	}
	if strings.TrimSpace(a.apiKey) == "" {
		return result, errors.New("本机网关密钥为空，拒绝写入 DeepSeek Harness")
	}
	home, err := resolveDeepSeekHarnessHome()
	if err != nil {
		return result, err
	}
	result.Home = home
	result.SettingsPath = filepath.Join(home, "settings.yaml")
	result.CredentialsPath = filepath.Join(home, ".credentials.yaml")

	settingsRaw, settingsExisted, err := readOptionalFile(result.SettingsPath)
	if err != nil {
		return result, fmt.Errorf("读取 DeepSeek Harness settings.yaml: %w", err)
	}
	credentialsRaw, credentialsExisted, err := readOptionalFile(result.CredentialsPath)
	if err != nil {
		return result, fmt.Errorf("读取 DeepSeek Harness .credentials.yaml: %w", err)
	}
	settingsDoc, err := parseYAMLMapping(settingsRaw, result.SettingsPath)
	if err != nil {
		return result, err
	}
	credentialsDoc, err := parseYAMLMapping(credentialsRaw, result.CredentialsPath)
	if err != nil {
		return result, err
	}
	if err := mergeDeepSeekHarnessSettings(settingsDoc, baseURL, model, imageInput); err != nil {
		return result, err
	}
	if err := mergeDeepSeekHarnessCredentials(credentialsDoc, a.apiKey); err != nil {
		return result, err
	}
	settingsNext, err := yaml.Marshal(settingsDoc)
	if err != nil {
		return result, fmt.Errorf("生成 DeepSeek Harness settings.yaml: %w", err)
	}
	credentialsNext, err := yaml.Marshal(credentialsDoc)
	if err != nil {
		return result, fmt.Errorf("生成 DeepSeek Harness .credentials.yaml: %w", err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return result, fmt.Errorf("创建 DeepSeek Harness 配置目录: %w", err)
	}
	backupDir := filepath.Join(home, "backups", "zcode-antigravity")
	stamp := a.now().UTC().Format("20060102-150405.000000000")
	if settingsExisted {
		result.SettingsBackup, err = writeDeepSeekHarnessBackup(backupDir, stamp+"-settings.yaml", settingsRaw)
		if err != nil {
			return result, err
		}
	}
	if credentialsExisted {
		result.CredentialsBackup, err = writeDeepSeekHarnessBackup(backupDir, stamp+"-credentials.yaml", credentialsRaw)
		if err != nil {
			return result, err
		}
	}
	if err := writeAtomic(result.CredentialsPath, credentialsNext, 0o600); err != nil {
		return result, fmt.Errorf("写入 DeepSeek Harness 凭据: %w", err)
	}
	if err := writeAtomic(result.SettingsPath, settingsNext, 0o600); err != nil {
		_ = restoreOptionalFile(result.CredentialsPath, credentialsRaw, credentialsExisted)
		return result, fmt.Errorf("写入 DeepSeek Harness 设置（凭据已回滚）: %w", err)
	}
	return result, nil
}

func resolveDeepSeekHarnessHome() (string, error) {
	home := strings.TrimSpace(os.Getenv("DSH_HOME"))
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("定位 DeepSeek Harness 用户目录: %w", err)
		}
		home = filepath.Join(userHome, ".dsh")
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return "", fmt.Errorf("解析 DSH_HOME: %w", err)
	}
	if filepath.Clean(abs) == filepath.VolumeName(abs)+string(os.PathSeparator) {
		return "", errors.New("DSH_HOME 不能是磁盘根目录")
	}
	return abs, nil
}

func readOptionalFile(path string) ([]byte, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return raw, err == nil, err
}

func parseYAMLMapping(raw []byte, path string) (*yaml.Node, error) {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return doc, nil
	}
	if err := yaml.Unmarshal(raw, doc); err != nil {
		return nil, fmt.Errorf("%s 不是有效 YAML，不会覆盖: %w", path, err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s 顶层必须是 YAML 对象，不会覆盖", path)
	}
	return doc, nil
}

func mergeDeepSeekHarnessSettings(doc *yaml.Node, baseURL, model string, imageInput bool) error {
	root := doc.Content[0]
	llm := ensureYAMLMapping(root, "llm-pi-ai")
	if llm == nil {
		return errors.New("DeepSeek Harness settings.yaml 的 llm-pi-ai 不是对象，不会覆盖")
	}
	providers := ensureYAMLMapping(llm, "providers")
	if providers == nil {
		return errors.New("DeepSeek Harness settings.yaml 的 llm-pi-ai.providers 不是对象，不会覆盖")
	}
	if existing, ok := yamlMappingValue(providers, deepSeekHarnessProviderID); ok {
		if existing.Kind != yaml.MappingNode {
			return errors.New("DeepSeek Harness 已有 zcode-bridge，但它不是对象，不会覆盖")
		}
		name, _ := yamlScalarValue(existing, "displayName")
		endpoint, _ := yamlScalarValue(existing, "baseURL")
		if name != deepSeekHarnessProviderName && !isLoopbackDeepSeekHarnessEndpoint(endpoint) {
			return errors.New("DeepSeek Harness 已存在非本程序创建的 zcode-bridge；请先改名或删除后重试")
		}
	}
	input := yamlSequence("text")
	if imageInput {
		input.Content = append(input.Content, yamlScalar("image"))
	}
	provider := yamlMapping(
		"displayName", yamlScalar(deepSeekHarnessProviderName),
		"apiKeyEnv", yamlScalar(deepSeekHarnessCredentialRef),
		"api", yamlScalar("openai-completions"),
		"baseURL", yamlScalar(baseURL),
		"compat", yamlMapping(
			"supportsDeveloperRole", yamlBool(false),
			"maxTokensField", yamlScalar("max_tokens"),
		),
		"models", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{
			yamlMapping("id", yamlScalar(model), "name", yamlScalar(model), "input", input),
		}},
	)
	setYAMLMappingValue(providers, deepSeekHarnessProviderID, provider)
	setYAMLMappingValue(root, "agent-default-model", yamlMapping(
		"provider", yamlScalar(deepSeekHarnessProviderID),
		"model", yamlScalar(model),
	))
	return nil
}

func mergeDeepSeekHarnessCredentials(doc *yaml.Node, apiKey string) error {
	root := doc.Content[0]
	if len(root.Content) == 0 {
		setYAMLMappingValue(root, "version", yamlInt(1))
	} else {
		version, ok := yamlScalarValue(root, "version")
		if !ok || version != "1" {
			return errors.New("DeepSeek Harness .credentials.yaml 不是 version: 1 格式，不会覆盖")
		}
	}
	refs := ensureYAMLMapping(root, "refs")
	if refs == nil {
		return errors.New("DeepSeek Harness .credentials.yaml 的 refs 不是对象，不会覆盖")
	}
	setYAMLMappingValue(refs, deepSeekHarnessCredentialRef, yamlScalar(apiKey))
	return nil
}

func ensureYAMLMapping(parent *yaml.Node, key string) *yaml.Node {
	if value, ok := yamlMappingValue(parent, key); ok {
		if value.Kind != yaml.MappingNode {
			return nil
		}
		return value
	}
	value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setYAMLMappingValue(parent, key, value)
	return value
}

func yamlMappingValue(parent *yaml.Node, key string) (*yaml.Node, bool) {
	if parent == nil || parent.Kind != yaml.MappingNode {
		return nil, false
	}
	for index := 0; index+1 < len(parent.Content); index += 2 {
		if parent.Content[index].Value == key {
			return parent.Content[index+1], true
		}
	}
	return nil, false
}

func yamlScalarValue(parent *yaml.Node, key string) (string, bool) {
	value, ok := yamlMappingValue(parent, key)
	return valueString(value), ok && value.Kind == yaml.ScalarNode
}

func valueString(value *yaml.Node) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(value.Value)
}

func setYAMLMappingValue(parent *yaml.Node, key string, value *yaml.Node) {
	for index := 0; index+1 < len(parent.Content); index += 2 {
		if parent.Content[index].Value == key {
			parent.Content[index+1] = value
			return
		}
	}
	parent.Content = append(parent.Content, yamlScalar(key), value)
}

func yamlMapping(values ...any) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for index := 0; index+1 < len(values); index += 2 {
		setYAMLMappingValue(node, values[index].(string), values[index+1].(*yaml.Node))
	}
	return node
}

func yamlSequence(values ...string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		node.Content = append(node.Content, yamlScalar(value))
	}
	return node
}

func yamlScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func yamlBool(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}
}

func yamlInt(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}

func isLoopbackDeepSeekHarnessEndpoint(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "http" && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost")
}

func writeDeepSeekHarnessBackup(dir, name string, raw []byte) (string, error) {
	path := filepath.Join(dir, name)
	if err := writeAtomic(path, raw, 0o600); err != nil {
		return "", fmt.Errorf("备份 DeepSeek Harness 配置: %w", err)
	}
	return path, nil
}

func restoreOptionalFile(path string, raw []byte, existed bool) error {
	if existed {
		return writeAtomic(path, raw, 0o600)
	}
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
