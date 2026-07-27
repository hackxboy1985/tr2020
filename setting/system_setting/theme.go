package system_setting

import (
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ThemeSettings struct {
	Frontend string `json:"frontend"`
}

var themeSettings = ThemeSettings{
	Frontend: "classic",
}

// themeEnvOverride is set at startup from FRONTEND_THEME env var.
// When non-empty, it takes precedence over the database config.
var themeEnvOverride string

func init() {
	if v := os.Getenv("FRONTEND_THEME"); v == "default" || v == "classic" {
		themeEnvOverride = v
	}
	config.GlobalConfig.Register("theme", &themeSettings)
	syncThemeToCommon()
}

func syncThemeToCommon() {
	if themeEnvOverride != "" {
		common.SetTheme(themeEnvOverride)
		return
	}
	common.SetTheme(themeSettings.Frontend)
}

func GetThemeSettings() *ThemeSettings {
	return &themeSettings
}

// UpdateAndSyncTheme syncs the theme config to common after DB load.
func UpdateAndSyncTheme() {
	syncThemeToCommon()
}
