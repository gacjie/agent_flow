package controller

import (
	"log/slog"
	"net/http"

	"agent_flow/src/middleware"
	"agent_flow/src/service"
)

// SysConfigCtrl 系统配置控制器
type SysConfigCtrl struct {
	Base
	ConfigService *service.SysConfigService
	ModelService  *service.LLMModelService
}

// Edit 系统配置页面
func (c *SysConfigCtrl) Edit(w http.ResponseWriter, r *http.Request) {
	configs, err := c.ConfigService.GetAll()
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取配置失败")
		return
	}

	data := map[string]interface{}{
		"Title":      "系统配置",
		"Configs":    configs,
		"AdminPage":  true,
		"ActiveMenu": "config",
	}

	if c.ModelService != nil {
		models, _ := c.ModelService.ListAll()
		data["Models"] = models
		visionModels, _ := c.ModelService.ListByCapability("vision")
		data["VisionModels"] = visionModels
		imageGenModels, _ := c.ModelService.ListByCapability("image_gen")
		data["ImageGenModels"] = imageGenModels
	}

	c.Render(w, r, "admin/config", data)
}

// Update 保存系统配置
func (c *SysConfigCtrl) Update(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	configs, _ := c.ConfigService.GetAll()
	kvs := make(map[string]string)
	for _, cfg := range configs {
		val := r.FormValue("config_" + cfg.Key)
		if val != cfg.Value {
			kvs[cfg.Key] = val
		}
	}

	if len(kvs) > 0 {
		if err := c.ConfigService.BatchUpdate(kvs); err != nil {
			c.RenderError(w, http.StatusInternalServerError, "保存配置失败")
			return
		}

		operator := middleware.GetCurrentAdmin(r)
		slog.Info("管理员操作:修改系统配置", "operator", operator.Username, "changes", kvs)
	}

	c.Redirect(w, r, "/admin/config")
}
