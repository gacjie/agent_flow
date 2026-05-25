package controller

import (
	"log/slog"
	"net/http"

	"agent_flow/src/middleware"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// PromptCtrl 系统提示词管理控制器
type PromptCtrl struct {
	Base
	PromptService *service.PromptService
}

// List 系统提示词列表页面
func (c *PromptCtrl) List(w http.ResponseWriter, r *http.Request) {
	items := c.PromptService.ListAll()
	data := map[string]interface{}{
		"Title":      "系统提示词",
		"Prompts":    items,
		"AdminPage":  true,
		"ActiveMenu": "prompts",
	}
	c.Render(w, r, "admin/prompt_list", data)
}

// Edit 编辑提示词页面
func (c *PromptCtrl) Edit(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	item, err := c.PromptService.GetOne(name)
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "提示词不存在")
		return
	}
	data := map[string]interface{}{
		"Title":      "编辑提示词 - " + item.Label,
		"Prompt":     item,
		"AdminPage":  true,
		"ActiveMenu": "prompts",
	}
	c.Render(w, r, "admin/prompt_edit", data)
}

// Update 保存提示词
func (c *PromptCtrl) Update(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}
	content := r.FormValue("content")
	if content == "" {
		c.RenderError(w, http.StatusBadRequest, "提示词内容不能为空")
		return
	}
	if err := c.PromptService.Save(name, content); err != nil {
		c.RenderError(w, http.StatusBadRequest, err.Error())
		return
	}
	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:编辑系统提示词", "operator", operator.Username, "name", name)
	c.Redirect(w, r, "/admin/prompts")
}

// Reset 重置提示词为默认值
func (c *PromptCtrl) Reset(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := c.PromptService.ResetToDefault(name); err != nil {
		c.RenderError(w, http.StatusBadRequest, err.Error())
		return
	}
	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:重置系统提示词", "operator", operator.Username, "name", name)
	c.Redirect(w, r, "/admin/prompts")
}
