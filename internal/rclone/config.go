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
		"parameters": toAnyMap(FilterCreateParameters(parameters)),
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

// OptionExample is a selectable value for an rclone provider option.
type OptionExample struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// ProviderOption describes one rclone backend configuration field.
type ProviderOption struct {
	Name      string          `json:"name"`
	Help      string          `json:"help,omitempty"`
	Default   string          `json:"default,omitempty"`
	Examples  []OptionExample `json:"examples,omitempty"`
	Required  bool            `json:"required"`
	Password  bool            `json:"password"`
	Type      string          `json:"type,omitempty"`
	Exclusive bool            `json:"exclusive"`
	Advanced  bool            `json:"advanced"`
	Hide      bool            `json:"hide"`
	Sensitive bool            `json:"sensitive"`
}

// Provider describes a supported rclone backend type and its options.
type Provider struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Options     []ProviderOption `json:"options,omitempty"`
}

// ListProviders returns supported rclone storage provider types with option schemas.
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
		name := stringField(entry, "Name", "name")
		if name == "" {
			continue
		}
		providers = append(providers, Provider{
			Name:        name,
			Description: stringField(entry, "Description", "description"),
			Options:     parseProviderOptions(entry["Options"], entry["options"]),
		})
	}
	return providers, nil
}

func parseProviderOptions(primary, fallback any) []ProviderOption {
	raw, ok := primary.([]any)
	if !ok {
		raw, ok = fallback.([]any)
	}
	if !ok {
		return nil
	}
	out := make([]ProviderOption, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		opt := ProviderOption{
			Name:      stringField(m, "Name", "name"),
			Help:      stringField(m, "Help", "help"),
			Default:   stringField(m, "Default", "default"),
			Type:      stringField(m, "Type", "type"),
			Required:  boolField(m, "Required", "required"),
			Password:  boolField(m, "Password", "password") || boolField(m, "IsPassword", "is_password"),
			Exclusive: boolField(m, "Exclusive", "exclusive"),
			Advanced:  boolField(m, "Advanced", "advanced"),
			Hide:      boolField(m, "Hide", "hide"),
			Sensitive: boolField(m, "Sensitive", "sensitive"),
		}
		if opt.Name == "" {
			continue
		}
		opt.Examples = parseOptionExamples(m["Examples"], m["examples"])
		out = append(out, opt)
	}
	return out
}

func parseOptionExamples(primary, fallback any) []OptionExample {
	raw, ok := primary.([]any)
	if !ok {
		raw, ok = fallback.([]any)
	}
	if !ok {
		return nil
	}
	out := make([]OptionExample, 0, len(raw))
	for _, item := range raw {
		switch v := item.(type) {
		case map[string]any:
			value := stringField(v, "Value", "value")
			if value == "" {
				value = fmt.Sprint(v)
			}
			out = append(out, OptionExample{
				Value:       value,
				Description: stringField(v, "Description", "description"),
			})
		case string:
			if v != "" {
				out = append(out, OptionExample{Value: v})
			}
		}
	}
	return out
}

func stringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			s := fmt.Sprint(v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func boolField(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				return t == "true" || t == "1"
			}
		}
	}
	return false
}
