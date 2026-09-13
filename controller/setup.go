package controller

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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

func workspaceInitialized(c *gin.Context) bool {
	return model.GetSetup(c.Request.Context()) != nil
}

func GetSetup(c *gin.Context) {
	setup := Setup{
		Status:       workspaceInitialized(c),
		DatabaseType: string(common.MainDatabaseType()),
	}
	if setup.Status {
		c.JSON(200, gin.H{"success": true, "data": setup})
		return
	}
	setup.RootInit = model.RootUserExists(c.Request.Context())
	c.JSON(200, gin.H{"success": true, "data": setup})
}

func PostSetup(c *gin.Context) {
	if workspaceInitialized(c) {
		c.JSON(200, gin.H{"success": false, "message": "系统已经初始化完成"})
		return
	}

	rootExists := model.RootUserExists(c.Request.Context())
	var req SetupRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(200, gin.H{"success": false, "message": "请求参数有误"})
		return
	}

	if !rootExists {
		username := strings.TrimSpace(req.Username)
		if username == "" || utf8.RuneCountInString(username) > model.UserNameMaxLength {
			c.JSON(200, gin.H{"success": false, "message": "用户名长度不能超过20个字符"})
			return
		}
		if req.Password != req.ConfirmPassword {
			c.JSON(200, gin.H{"success": false, "message": "两次输入的密码不一致"})
			return
		}
		if err := common.ValidateNewAccountPassword(req.Password); err != nil {
			c.JSON(200, gin.H{"success": false, "message": err.Error()})
			return
		}
		hashedPassword, err := common.HashAccountPassword(req.Password)
		if err != nil {
			c.JSON(200, gin.H{"success": false, "message": "系统错误: " + err.Error()})
			return
		}
		rootUser := model.User{
			Username:    username,
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			Quota:       0,
			AuthVersion: 1,
		}
		if err := model.DB.WithContext(c.Request.Context()).Create(&rootUser).Error; err != nil {
			c.JSON(200, gin.H{"success": false, "message": "创建管理员账号失败: " + err.Error()})
			return
		}
	}

	if err := model.UpdateOption(c.Request.Context(), "SelfUseModeEnabled", boolToString(req.SelfUseModeEnabled)); err != nil {
		c.JSON(200, gin.H{"success": false, "message": "保存自用模式设置失败: " + err.Error()})
		return
	}
	if err := model.UpdateOption(c.Request.Context(), "DemoSiteEnabled", boolToString(req.DemoSiteEnabled)); err != nil {
		c.JSON(200, gin.H{"success": false, "message": "保存演示站点模式设置失败: " + err.Error()})
		return
	}

	setup := model.Setup{Version: common.Version, InitializedAt: time.Now().Unix()}
	if err := model.DB.WithContext(c.Request.Context()).Create(&setup).Error; err != nil {
		c.JSON(200, gin.H{"success": false, "message": "系统初始化失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "message": "系统初始化成功"})
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
