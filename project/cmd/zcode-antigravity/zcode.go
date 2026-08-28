package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const zcodeWebSearchModelID = "gemini-web-search"

var zcodeModelAllowlist = []string{
	"gemini-3.7-flash",
	"gemini-3.6-flash",
	zcodeWebSearchModelID,
}

type zcodeModelAlias struct {
	UpstreamID  string
	ClientID    string
	DisplayName string
}

var zcodeModelAliases = []zcodeModelAlias{
	{UpstreamID: "gemini-3.7-flash-high", ClientID: "gemini-3.7-flash", DisplayName: "Gemini 3.7 Flash"},
	{UpstreamID: "gemini-3.6-flash-high", ClientID: "gemini-3.6-flash", DisplayName: "Gemini 3.6 Flash"},
	{UpstreamID: "gemini-3.1-flash-lite", ClientID: zcodeWebSearchModelID, DisplayName: "Gemini Web Search (Google)"},
}

func (a *app) configureZCode(port int, models []modelInfo) (backup string, changed bool, err error) {
	return a.configureZCodeWithAccess(port, models, false, false)
}

func (a *app) configureZCodeWithAccess(port int, models []modelInfo, includeGrok, includeOther bool) (backup string, changed bool, err error) {
	if len(models) == 0 {
		return "", false, fmt.Errorf("没有可写入 ZCode 的模型")
	}
	includeGemini, hasGrok, hasOther := false, false, false
	for _, model := range models {
		if isAllowedZCodeModel(model.ID) {
			includeGemini = true
		}
		if isXAITextModel(model) {
			hasGrok = true
		}
		if isOtherTextModel(model) {
			hasOther = true
		}
	}
	models, err = selectAgentModels(models, includeGemini, includeGrok && hasGrok, includeOther && hasOther)
	if err != nil {
		return "", false, err
	}
	raw, root, err := readJSONObject(a.paths.ZCodeConfig)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, fmt.Errorf("找不到 %s；请先安装并运行一次 ZCode，再执行 sync 命令或平台同步脚本", a.paths.ZCodeConfig)
		}
		return "", false, err
	}
	providers, err := objectField(root, "provider")
	if err != nil {
		return "", false, err
	}
	if existing, exists := providers[providerID]; exists && !isManagedProvider(existing) {
		return "", false, fmt.Errorf("ZCode 已存在同名 Provider %q，但它不是本程序创建的；为避免覆盖用户配置已停止", providerID)
	}
	modelMap := make(map[string]any, len(models))
	for index, model := range models {
		contextLimit := model.MaxInputTokens
		if contextLimit <= 0 {
			contextLimit = 200000
		}
		inputModalities := normalizedModalities(model.SupportedInputModalities)
		outputModalities := normalizedModalities(model.SupportedOutputModalities)
		entry := map[string]any{
			"limit": map[string]any{
				"context": contextLimit,
			},
			"modalities": map[string]any{
				"input":  inputModalities,
				"output": outputModalities,
			},
			"zcode": map[string]any{
				"modified": false,
				"priority": 200 + index,
			},
		}
		if model.DisplayName != "" && model.DisplayName != model.ID {
			entry["name"] = model.DisplayName
		}
		if variants := zcodeReasoningVariants(model); len(variants) > 0 {
			entry["reasoning"] = map[string]any{
				"enabled":        true,
				"variants":       variants,
				"defaultVariant": variants[len(variants)-1],
			}
		}
		modelMap[model.ID] = entry
	}
	provider := map[string]any{
		"name": providerName,
		"kind": "anthropic",
		"options": map[string]any{
			"apiKey":         a.apiKey,
			"baseURL":        a.gatewayURL(port),
			"apiKeyRequired": true,
		},
		"enabled":                     true,
		"source":                      "custom",
		"models":                      modelMap,
		"x-zcode-antigravity-managed": 1,
	}
	if existing, ok := providers[providerID]; ok && jsonSemanticallyEqual(existing, provider) {
		return "", false, nil
	}
	if a.zcodeRunning != nil && a.zcodeRunning() {
		return "", false, fmt.Errorf("检测到 ZCode 仍在运行；Provider 需要更新，请彻底退出 ZCode 后重试同步")
	}
	backup, err = a.backupZCodeConfig(raw, "before-sync")
	if err != nil {
		return "", false, fmt.Errorf("备份 ZCode 配置失败，未修改原文件: %w", err)
	}
	providers[providerID] = provider
	root["provider"] = providers
	encoded, err := marshalJSONObject(root)
	if err != nil {
		return backup, false, err
	}
	if err := writeAtomic(a.paths.ZCodeConfig, encoded, 0o600); err != nil {
		return backup, false, fmt.Errorf("写入 ZCode 配置失败；原文件备份在 %s: %w", backup, err)
	}
	_, verifiedRoot, verifyErr := readJSONObject(a.paths.ZCodeConfig)
	if verifyErr != nil {
		return backup, false, fmt.Errorf("写后校验失败；请从备份 %s 手工恢复: %w", backup, verifyErr)
	}
	verifiedProviders, verifyErr := objectField(verifiedRoot, "provider")
	if verifyErr != nil || !jsonSemanticallyEqual(verifiedProviders[providerID], provider) {
		return backup, false, fmt.Errorf("写后校验失败；请从备份 %s 手工恢复", backup)
	}
	return backup, true, nil
}

func selectZCodeModels(catalog []modelInfo) ([]modelInfo, error) {
	return selectAgentModels(catalog, true, false, false)
}

func selectAgentModels(catalog []modelInfo, includeGemini, includeGrok, includeOther bool) ([]modelInfo, error) {
	byID := make(map[string]modelInfo, len(catalog))
	for _, model := range catalog {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		model.ID = id
		byID[id] = model
	}
	selected := make([]modelInfo, 0, len(zcodeModelAllowlist)+8)
	if includeGemini {
		missing := make([]string, 0)
		for _, id := range zcodeModelAllowlist {
			model, ok := byID[id]
			if !ok {
				missing = append(missing, id)
				continue
			}
			selected = append(selected, model)
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("Antigravity 当前账号缺少必须模型: %s；请更新本程序、重启网关并确认账号具有对应模型权限", strings.Join(missing, ", "))
		}
	}
	if includeGrok {
		grok := make([]modelInfo, 0, len(catalog))
		for _, model := range catalog {
			if isXAITextModel(model) {
				grok = append(grok, model)
			}
		}
		sort.Slice(grok, func(i, j int) bool { return grok[i].ID < grok[j].ID })
		if len(grok) == 0 {
			return nil, fmt.Errorf("xAI 账号已登录，但模型目录中没有可用的 Grok 文本模型")
		}
		selected = append(selected, grok...)
	}
	if includeOther {
		other := make([]modelInfo, 0, len(catalog))
		for _, model := range catalog {
			if isOtherTextModel(model) {
				other = append(other, model)
			}
		}
		sort.Slice(other, func(i, j int) bool { return other[i].ID < other[j].ID })
		selected = append(selected, other...)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("没有已登录的模型提供商")
	}
	return selected, nil
}

// selectAvailableAgentModels keeps the two providers independent: a temporary
// catalog or entitlement issue on one side must not make the other provider
// unusable. Callers can surface warnings while continuing with valid models.
func selectAvailableAgentModels(catalog []modelInfo, includeAntigravity, includeGrok, includeOther bool) ([]modelInfo, []error) {
	selected := make([]modelInfo, 0, len(zcodeModelAllowlist)+8)
	warnings := make([]error, 0, 2)
	if includeAntigravity {
		models, err := selectAgentModels(catalog, true, false, includeOther)
		if err != nil {
			warnings = append(warnings, err)
		} else {
			selected = append(selected, models...)
		}
	}
	if includeGrok {
		models, err := selectAgentModels(catalog, false, true, false)
		if err != nil {
			warnings = append(warnings, err)
		} else {
			selected = append(selected, models...)
		}
	}
	if !includeAntigravity && !includeGrok {
		warnings = append(warnings, fmt.Errorf("没有已启用且已登录的模型提供商；默认仅启用 Gemini，可在设置中打开 Grok"))
	}
	return selected, warnings
}

func isXAITextModel(model modelInfo) bool {
	id := strings.ToLower(strings.TrimSpace(model.ID))
	if !strings.HasPrefix(id, "grok-") || strings.Contains(id, "imagine") || strings.Contains(id, "image") || strings.Contains(id, "video") {
		return false
	}
	for _, modality := range model.SupportedOutputModalities {
		value := strings.ToLower(strings.TrimSpace(modality))
		if value != "" && value != "text" {
			return false
		}
	}
	return true
}

func isOtherTextModel(model modelInfo) bool {
	id := strings.ToLower(strings.TrimSpace(model.ID))
	if id == "" || strings.HasPrefix(id, "gemini-") || isXAITextModel(model) {
		return false
	}
	for _, marker := range []string{"image", "imagine", "video", "audio", "embed", "tts"} {
		if strings.Contains(id, marker) {
			return false
		}
	}
	for _, modality := range model.SupportedOutputModalities {
		value := strings.ToLower(strings.TrimSpace(modality))
		if value != "" && value != "text" {
			return false
		}
	}
	return true
}

func isManagedAgentModel(model modelInfo) bool {
	return isAllowedZCodeModel(model.ID) || isXAITextModel(model) || isOtherTextModel(model)
}

func isAllowedZCodeModel(id string) bool {
	id = strings.TrimSpace(id)
	for _, allowed := range zcodeModelAllowlist {
		if id == allowed {
			return true
		}
	}
	return false
}

func zcodeReasoningVariants(model modelInfo) []string {
	if model.Thinking == nil || len(model.Thinking.Levels) == 0 {
		return nil
	}
	supported := make(map[string]bool, len(model.Thinking.Levels))
	for _, level := range model.Thinking.Levels {
		supported[strings.ToLower(strings.TrimSpace(level))] = true
	}
	// Match Antigravity's Gemini 3.7 selector. "minimal" is accepted by the
	// upstream model but is not presented by the Antigravity desktop UI.
	variants := make([]string, 0, 3)
	for _, level := range []string{"low", "medium", "high"} {
		if supported[level] {
			variants = append(variants, level)
		}
	}
	return variants
}

func normalizedModalities(values []string) []string {
	allowed := map[string]bool{"text": true, "image": true, "audio": true, "video": true}
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !allowed[value] || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	if len(result) == 0 {
		return []string{"text"}
	}
	return result
}

func jsonSemanticallyEqual(left, right any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}

func (a *app) removeZCodeProvider() error {
	if a.zcodeRunning != nil && a.zcodeRunning() {
		return fmt.Errorf("检测到 ZCode 仍在运行；请彻底退出后再删除 Provider")
	}
	raw, root, err := readJSONObject(a.paths.ZCodeConfig)
	if err != nil {
		return err
	}
	providers, err := objectField(root, "provider")
	if err != nil {
		return err
	}
	if _, exists := providers[providerID]; !exists {
		fmt.Println("ZCode 中没有本程序的 Provider，无需删除。")
		return nil
	}
	if !isManagedProvider(providers[providerID]) {
		return fmt.Errorf("同名 Provider 没有本程序管理标记，拒绝删除")
	}
	backup, err := a.backupZCodeConfig(raw, "before-remove")
	if err != nil {
		return fmt.Errorf("备份失败，未修改 ZCode: %w", err)
	}
	delete(providers, providerID)
	root["provider"] = providers
	encoded, err := marshalJSONObject(root)
	if err != nil {
		return err
	}
	if err := writeAtomic(a.paths.ZCodeConfig, encoded, 0o600); err != nil {
		return fmt.Errorf("删除 Provider 失败；备份在 %s: %w", backup, err)
	}
	fmt.Printf("已只删除 %s；其他 ZCode 配置未删除。\n", providerName)
	fmt.Printf("删除前备份: %s\n", backup)
	return nil
}

func isManagedProvider(value any) bool {
	provider, ok := value.(map[string]any)
	if !ok {
		return false
	}
	marker, exists := provider["x-zcode-antigravity-managed"]
	if !exists {
		return false
	}
	switch typed := marker.(type) {
	case json.Number:
		return typed.String() == "1"
	case float64:
		return typed == 1
	case int:
		return typed == 1
	default:
		return false
	}
}

func (a *app) backupZCodeConfig(raw []byte, reason string) (string, error) {
	if err := os.MkdirAll(a.paths.ZCodeBackups, 0o700); err != nil {
		return "", err
	}
	stamp := a.now().Format("20060102-150405.000000000")
	for attempt := 0; attempt < 100; attempt++ {
		suffix := ""
		if attempt > 0 {
			suffix = fmt.Sprintf("-%02d", attempt)
		}
		name := fmt.Sprintf("config-%s-%s%s.json", stamp, reason, suffix)
		path := filepath.Join(a.paths.ZCodeBackups, name)
		if err := writeFileExclusive(path, raw, 0o600); err == nil {
			if errPrune := pruneManagedBackups(a.paths.ZCodeBackups, 20); errPrune != nil {
				return "", fmt.Errorf("备份已创建但清理旧备份失败: %w", errPrune)
			}
			return path, nil
		} else if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("同一时间戳的备份文件过多")
}

func readJSONObject(path string) ([]byte, map[string]any, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("%s 是目录，不是 JSON 文件", path)
	}
	if info.Size() > 32<<20 {
		return nil, nil, fmt.Errorf("%s 超过 32 MiB，拒绝自动修改", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return raw, nil, fmt.Errorf("%s 不是有效 JSON（不会自动覆盖）: %w", path, err)
	}
	if root == nil {
		return raw, nil, fmt.Errorf("%s 顶层必须是 JSON 对象", path)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return raw, nil, fmt.Errorf("%s 包含多个 JSON 值", path)
		}
		return raw, nil, fmt.Errorf("%s 包含多余 JSON 内容: %w", path, err)
	}
	return raw, root, nil
}

func objectField(root map[string]any, key string) (map[string]any, error) {
	value, exists := root[key]
	if !exists || value == nil {
		return map[string]any{}, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("ZCode config.json 的 %q 字段不是对象，拒绝覆盖", key)
	}
	return object, nil
}

func marshalJSONObject(root map[string]any) ([]byte, error) {
	raw, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func zcodeProviderStatus(path string) (configured bool, baseURL string, err error) {
	_, root, err := readJSONObject(path)
	if err != nil {
		return false, "", err
	}
	providers, err := objectField(root, "provider")
	if err != nil {
		return false, "", err
	}
	rawProvider, ok := providers[providerID]
	if !ok {
		return false, "", nil
	}
	if !isManagedProvider(rawProvider) {
		return false, "", fmt.Errorf("同名 Provider 不带本程序管理标记")
	}
	provider, ok := rawProvider.(map[string]any)
	if !ok {
		return false, "", fmt.Errorf("本 Provider 配置类型异常")
	}
	options, _ := provider["options"].(map[string]any)
	baseURL, _ = options["baseURL"].(string)
	return true, baseURL, nil
}

func sortedProviderModelIDs(provider map[string]any) []string {
	models, _ := provider["models"].(map[string]any)
	ids := make([]string, 0, len(models))
	for id := range models {
		if strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
