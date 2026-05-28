package rclone

import (
	"encoding/json"
	"fmt"
)

var configRPC rpcCaller = librcloneCaller{}

// ListRemotes returns configured rclone remote names.
func ListRemotes() ([]string, error) {
	out, err := configRPC.call("config/listremotes", nil)
	if err != nil {
		return nil, err
	}
	raw, ok := out["remotes"].([]any)
	if !ok {
		return nil, nil
	}
	remotes := make([]string, 0, len(raw))
	for _, item := range raw {
		name, ok := item.(string)
		if ok && name != "" {
			remotes = append(remotes, name)
		}
	}
	return remotes, nil
}

// ShowRemote returns the configuration map for a remote.
func ShowRemote(name string) (map[string]string, error) {
	out, err := configRPC.call("config/get", map[string]any{"name": name})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(out))
	for key, value := range out {
		result[key] = fmt.Sprint(value)
	}
	return result, nil
}

// CreateRemote creates a new rclone remote with generic key=value parameters.
func CreateRemote(name, remoteType string, parameters map[string]string) error {
	params := map[string]any{
		"name":       name,
		"type":       remoteType,
		"parameters": toAnyMap(parameters),
		"opt": map[string]any{
			"nonInteractive": true,
			"obscure":        true,
		},
	}
	_, err := configRPC.call("config/create", params)
	return err
}

// UpdateRemote updates an existing rclone remote.
func UpdateRemote(name string, parameters map[string]string) error {
	params := map[string]any{
		"name":       name,
		"parameters": toAnyMap(parameters),
		"opt": map[string]any{
			"nonInteractive": true,
			"obscure":        true,
		},
	}
	_, err := configRPC.call("config/update", params)
	return err
}

// DeleteRemote removes a configured rclone remote.
func DeleteRemote(name string) error {
	_, err := configRPC.call("config/delete", map[string]any{"name": name})
	return err
}

func toAnyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// FormatRemoteConfig returns a pretty JSON representation of remote config.
func FormatRemoteConfig(cfg map[string]string) (string, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Provider describes a supported rclone backend type.
type Provider struct {
	Name        string
	Description string
}

// ListProviders returns supported rclone storage provider types.
func ListProviders() ([]Provider, error) {
	out, err := configRPC.call("config/providers", nil)
	if err != nil {
		return nil, err
	}
	raw, ok := out["providers"].([]any)
	if !ok {
		return nil, nil
	}
	providers := make([]Provider, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["Name"].(string)
		if name == "" {
			continue
		}
		desc, _ := entry["Description"].(string)
		providers = append(providers, Provider{Name: name, Description: desc})
	}
	return providers, nil
}
