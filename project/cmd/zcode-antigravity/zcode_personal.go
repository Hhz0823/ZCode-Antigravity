package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const geminiHighReasoningMap = `reasoningLevel == "disabled" ? {"thinking":{"type":"disabled"},"output_config":null} : {"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`

type zcodeConfigWrite struct {
	path, reason  string
	before, after []byte
	existed       bool
}

// Current ZCode reads provider_config.json; config.json remains necessary for older clients.
func (a *app) preparePersonalProvider(port int, models []modelInfo, owned bool) (zcodeConfigWrite, error) {
	update := zcodeConfigWrite{path: filepath.Join(filepath.Dir(a.paths.ZCodeConfig), "provider_config.json"), reason: "provider-before-sync"}
	raw, root, err := readJSONObject(update.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return update, err
	}
	update.before, update.existed = raw, err == nil
	if !update.existed {
		if len(models) == 0 {
			return update, nil
		}
		root = map[string]any{"schemaVersion": 1, "config": map[string]any{}}
	}
	if fmt.Sprint(root["schemaVersion"]) != "1" {
		return update, fmt.Errorf("不支持的 ZCode provider_config.json 版本，未修改配置")
	}
	original, err := marshalJSONObject(root)
	if err != nil {
		return update, err
	}
	config, err := objectField(root, "config")
	if err != nil {
		return update, err
	}
	providers, err := objectField(config, "providerConfigRules")
	if err != nil {
		return update, err
	}
	modelRules, err := objectField(config, "modelConfigRules")
	if err != nil {
		return update, err
	}
	for _, item := range []struct {
		root map[string]any
		key  string
	}{{providers, "providerRules"}, {modelRules, "providerModelRules"}, {modelRules, "manualProviderModelRules"}} {
		kept := []any{}
		if value := item.root[item.key]; value != nil {
			rules, ok := value.([]any)
			if !ok {
				return update, fmt.Errorf("ZCode %s 不是数组，拒绝覆盖", item.key)
			}
			for _, value := range rules {
				rule, ok := value.(map[string]any)
				if !ok {
					return update, fmt.Errorf("ZCode %s 包含无效规则，拒绝覆盖", item.key)
				}
				if rule["providerId"] == providerID {
					if !owned {
						return update, fmt.Errorf("新版 ZCode 存在同名但无法确认归属的 Provider，拒绝覆盖")
					}
					continue
				}
				kept = append(kept, rule)
			}
		}
		item.root[item.key] = kept
	}
	selection, err := objectField(config, "defaultModelSelection")
	if err != nil {
		return update, err
	}
	if len(models) > 0 {
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
			limit := model.MaxInputTokens
			if limit <= 0 {
				limit = 200000
			}
			if isAllowedZCodeModel(model.ID) && (model.MaxInputTokens <= 0 || limit > geminiContextLimit) {
				limit = geminiContextLimit
			}
			properties := map[string]any{"contextWindow": limit}
			input := map[string]any{"supportsText": true, "supportsImage": false, "supportsAudio": false, "supportsVideo": false, "supportsPdf": false}
			for _, modality := range normalizedModalities(model.SupportedInputModalities) {
				switch modality {
				case "image":
					input["supportsImage"] = true
				case "audio":
					input["supportsAudio"] = true
				case "video":
					input["supportsVideo"] = true
				}
			}
			properties["inputFormat"] = input
			modelConfig := map[string]any{"enabled": true, "properties": properties}
			if isAllowedZCodeModel(model.ID) {
				modelConfig["optionSpecs"] = map[string]any{"reasoningLevel": map[string]any{"values": []string{"disabled", "enabled"}, "map": geminiHighReasoningMap}}
			}
			modelRules["providerModelRules"] = append(modelRules["providerModelRules"].([]any), map[string]any{"providerId": providerID, "modelId": model.ID, "config": modelConfig})
		}
		providers["providerRules"] = append(providers["providerRules"].([]any), map[string]any{
			"providerId": providerID, "providerName": providerName, "enabled": true,
			"config": map[string]any{"group": "standard-personal", "access": map[string]any{"type": "api-key", "apiKey": a.apiKey}, "api": map[string]any{"type": "anthropic-messages", "baseUrl": a.gatewayURL(port)}, "personalModelIds": ids, "modelOrder": ids},
		})
		if len(selection) == 0 || selection["providerId"] == providerID {
			selected := models[0].ID
			for _, id := range ids {
				if selection["modelId"] == id {
					selected = id
					break
				}
			}
			config["defaultModelSelection"] = map[string]any{"providerId": providerID, "modelId": selected}
			if isAllowedZCodeModel(selected) {
				config["defaultModelSelection"].(map[string]any)["options"] = map[string]any{"reasoningLevel": "enabled"}
			}
		}
	} else {
		if selection["providerId"] == providerID {
			delete(config, "defaultModelSelection")
		}
		if order, ok := config["providerOrder"].([]any); ok {
			kept := []any{}
			for _, id := range order {
				if id != providerID {
					kept = append(kept, id)
				}
			}
			config["providerOrder"] = kept
		}
	}
	config["providerConfigRules"], config["modelConfigRules"] = providers, modelRules
	root["config"] = config
	update.after, err = marshalJSONObject(root)
	if err != nil {
		return update, err
	}
	if update.existed {
		if bytes.Equal(original, update.after) {
			update.after = nil
		}
	}
	return update, nil
}

// Prepare and back up both schemas before changing either. A failed write rolls back prior writes.
func (a *app) commitZCodeConfigs(updates ...zcodeConfigWrite) (string, error) {
	backup := ""
	for _, update := range updates {
		if update.after == nil || !update.existed {
			continue
		}
		path, err := a.backupZCodeConfig(update.before, update.reason)
		if err != nil {
			return backup, fmt.Errorf("备份 ZCode 配置失败，未修改原文件: %w", err)
		}
		if backup == "" {
			backup = path
		}
	}
	written := []zcodeConfigWrite{}
	for _, update := range updates {
		if update.after == nil {
			continue
		}
		written = append(written, update)
		err := writeAtomic(update.path, update.after, 0600)
		if err == nil {
			var actual []byte
			actual, err = os.ReadFile(update.path)
			if err == nil && !bytes.Equal(actual, update.after) {
				err = fmt.Errorf("ZCode 配置写后校验失败")
			}
		}
		if err != nil {
			for i := len(written) - 1; i >= 0; i-- {
				previous := written[i]
				var restoreErr error
				if previous.existed {
					restoreErr = writeAtomic(previous.path, previous.before, 0600)
				} else {
					restoreErr = os.Remove(previous.path)
					if errors.Is(restoreErr, fs.ErrNotExist) {
						restoreErr = nil
					}
				}
				err = errors.Join(err, restoreErr)
			}
			return backup, fmt.Errorf("写入 ZCode 配置失败，已尝试回退；备份 %s: %w", backup, err)
		}
	}
	return backup, nil
}
