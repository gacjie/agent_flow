package controller

import (
	"log/slog"
	"net/http"
	"strconv"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/middleware"
	"agent_flow/src/model"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// SkillCtrl 技能管理控制器（/admin/skills）
type SkillCtrl struct {
	Base
	Service *service.SkillService
}

// List 技能列表页面
func (c *SkillCtrl) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := config.Get().Pagination.DefaultPageSize

	items, total, err := c.Service.List(page, pageSize)
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取技能列表失败")
		return
	}

	paging := common.Pagination{
		Page:     page,
		PageSize: pageSize,
		Total:    int(total),
	}
	if paging.PageSize > 0 {
		paging.TotalPage = (paging.Total + paging.PageSize - 1) / paging.PageSize
	}

	data := map[string]interface{}{
		"Title":      "技能管理",
		"Skills":     items,
		"Paging":     paging,
		"AdminPage":  true,
		"ActiveMenu": "skills",
	}
	c.Render(w, r, "admin/skill_list", data)
}

// CreateForm 创建技能表单页面
func (c *SkillCtrl) CreateForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":      "创建技能",
		"Action":     "/admin/skills",
		"Method":     "POST",
		"FormItem":   model.Skill{Level: 1, Status: 1},
		"AdminPage":  true,
		"ActiveMenu": "skills",
	}
	c.Render(w, r, "admin/skill_form", data)
}

// Create 创建技能
func (c *SkillCtrl) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	level, _ := strconv.Atoi(r.FormValue("level"))
	if level < 1 || level > 3 {
		level = 1
	}

	req := &model.SkillCreateReq{
		Name:        r.FormValue("name"),
		Label:       r.FormValue("label"),
		Description: r.FormValue("description"),
		Keywords:    r.FormValue("keywords"),
		Content:     r.FormValue("content"),
		Level:       level,
	}

	newSkill, err := c.Service.Create(req)
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "创建技能失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:创建技能", "operator", operator.Username, "skill", newSkill.Name)

	c.Redirect(w, r, "/admin/skills")
}

// EditForm 编辑技能表单页面
func (c *SkillCtrl) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	skill, err := c.Service.GetByID(uint(id))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "技能不存在")
		return
	}

	data := map[string]interface{}{
		"Title":      "编辑技能",
		"Action":     "/admin/skills/" + chi.URLParam(r, "id"),
		"Method":     "PUT",
		"FormItem":   skill,
		"AdminPage":  true,
		"ActiveMenu": "skills",
	}
	c.Render(w, r, "admin/skill_form", data)
}

// Update 更新技能
func (c *SkillCtrl) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	level, _ := strconv.Atoi(r.FormValue("level"))
	status, _ := strconv.Atoi(r.FormValue("status"))
	req := &model.SkillUpdateReq{
		Label:       r.FormValue("label"),
		Description: r.FormValue("description"),
		Keywords:    r.FormValue("keywords"),
		Content:     r.FormValue("content"),
		Level:       &level,
		Status:      &status,
	}

	if _, err := c.Service.Update(uint(id), req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "更新技能失败")
		return
	}

	c.Redirect(w, r, "/admin/skills")
}

// Delete 删除技能
func (c *SkillCtrl) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := c.Service.Delete(uint(id)); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "删除技能失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:删除技能", "operator", operator.Username, "skill_id", id)

	common.JSONSuccess(w, nil)
}
