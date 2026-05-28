package rclone

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/rclone/rclone/librclone/librclone"
)

// rpcCaller executes rclone RC methods.
type rpcCaller interface {
	call(method string, params map[string]any) (map[string]any, error)
}

type librcloneCaller struct{}

func (librcloneCaller) call(method string, params map[string]any) (map[string]any, error) {
	ensureInitialized()

	var input string
	if len(params) > 0 {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal rclone rpc params: %w", err)
		}
		input = string(data)
	}

	output, status := librclone.RPC(method, input)
	if status != http.StatusOK {
		return nil, parseRPCError(method, output, status)
	}
	if output == "" {
		return map[string]any{}, nil
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		return nil, fmt.Errorf("parse rclone rpc response for %s: %w", method, err)
	}
	return out, nil
}

var (
	initOnce sync.Once
)

func ensureInitialized() {
	initOnce.Do(func() {
		librclone.Initialize()
	})
}

func parseRPCError(method, output string, status int) error {
	if output == "" {
		return fmt.Errorf("rclone rpc %s failed with status %d", method, status)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return fmt.Errorf("rclone rpc %s failed with status %d: %s", method, status, output)
	}
	if msg, ok := payload["error"].(string); ok && msg != "" {
		return fmt.Errorf("rclone rpc %s: %s", method, msg)
	}
	return fmt.Errorf("rclone rpc %s failed with status %d: %s", method, status, output)
}
