package controller

import (
	"github.com/gin-gonic/gin"
)

type Setup struct {
	Status       bool   `json:"status"`
	RootInit     bool   `json:"root_init"`
	DatabaseType string `json:"database_type"`
}

type SetupRequest struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	ConfirmPassword    string `json:"confirmPassword"`
	SelfUseModeEnabled bool   `json:"SelfUseModeEnabled"`
	DemoSiteEnabled    bool   `json:"DemoSiteEnabled"`
}

func GetSetup(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "data": Setup{Status: true, RootInit: true}})
}

func PostSetup(c *gin.Context) {
	c.JSON(409, gin.H{"success": false, "code": "workspace_already_initialized", "message": "Workspace initialization is managed by the platform"})
}
