package webui

import (
	"fmt"
	"net/http"

	"github.com/Origin173/SnapCraft/internal/config"
)

// ControlProfile returns "singleplayer" or "server" for the WebUI.
func ControlProfile(cfg *config.Config) string {
	if cfg.Server.Control.Type == config.ControlNone {
		return "singleplayer"
	}
	return "server"
}

// ApplyControlProfile updates cfg for the requested WebUI profile.
func ApplyControlProfile(cfg *config.Config, profile string) error {
	switch profile {
	case "singleplayer":
		cfg.Server.Control.Type = config.ControlNone
		return nil
	case "server":
		if cfg.Server.Control.Type == config.ControlNone {
			cfg.Server.Control.Type = config.ControlRCON
		}
		return nil
	default:
		return fmt.Errorf("profile must be %q or %q", "singleplayer", "server")
	}
}

func controlProfileLabel(profile string) string {
	switch profile {
	case "singleplayer":
		return "单人存档"
	case "server":
		return "多人服务器"
	default:
		return profile
	}
}

func (s *Server) handleSetControlMode(w http.ResponseWriter, r *http.Request) {
	if s.configPath == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("config path unknown; restart with --config"))
		return
	}
	var body struct {
		Profile string `json:"profile"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	cfg := s.getConfig()
	clone := *cfg
	if err := ApplyControlProfile(&clone, body.Profile); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Validate(&clone); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Save(s.configPath, &clone); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.setConfig(&clone)
	s.logInfo("config", "存档模式已切换", map[string]any{
		"profile":      body.Profile,
		"control_type": clone.Server.Control.Type,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"message":       fmt.Sprintf("已切换为%s模式", controlProfileLabel(body.Profile)),
		"profile":       ControlProfile(&clone),
		"control_type":  clone.Server.Control.Type,
	})
}
