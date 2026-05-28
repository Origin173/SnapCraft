package rclone

import "testing"

func TestParseProviderOptions(t *testing.T) {
	raw := []any{
		map[string]any{
			"Name":     "url",
			"Help":     "URL of http host to connect to",
			"Required": true,
			"Type":     "string",
		},
		map[string]any{
			"name":      "vendor",
			"help":      "Name of the WebDAV site/service/software you are using",
			"exclusive": true,
			"examples": []any{
				map[string]any{"Value": "nextcloud", "Description": "Nextcloud"},
				map[string]any{"Value": "other", "Description": "Other"},
			},
		},
		map[string]any{
			"Name":     "pass",
			"Password": true,
			"Hide":     false,
		},
	}
	opts := parseProviderOptions(raw, nil)
	if len(opts) != 3 {
		t.Fatalf("options = %d", len(opts))
	}
	if opts[0].Name != "url" || !opts[0].Required {
		t.Fatalf("url option = %#v", opts[0])
	}
	if len(opts[1].Examples) != 2 || opts[1].Examples[0].Value != "nextcloud" {
		t.Fatalf("vendor examples = %#v", opts[1].Examples)
	}
	if !opts[2].Password || opts[2].Name != "pass" {
		t.Fatalf("pass option = %#v", opts[2])
	}
}

func TestListProvidersSchema(t *testing.T) {
	rpc := &fakeRPC{response: map[string]any{
		"providers": []any{
			map[string]any{
				"Name":        "webdav",
				"Description": "WebDAV",
				"Options": []any{
					map[string]any{"Name": "url", "Required": true},
					map[string]any{"Name": "vendor", "Examples": []any{"nextcloud", "other"}},
				},
			},
		},
	}}
	old := configRPC
	configRPC = rpc
	t.Cleanup(func() { configRPC = old })

	providers, err := ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].Name != "webdav" {
		t.Fatalf("providers = %#v", providers)
	}
	if len(providers[0].Options) < 2 {
		t.Fatalf("webdav options = %#v", providers[0].Options)
	}
}
