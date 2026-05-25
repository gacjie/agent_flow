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

// Member 管理员管理控制器（/admin/members）
type Member struct {
	Base
	Service     *service.AdminService
	PermService *service.PermissionService
}

// List 管理员列表页面
func (c *Member) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := config.Get().Pagination.DefaultPageSize

	admins, total, err := c.Service.List(page, pageSize)
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取管理员列表失败")
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
		"Title":      "管理员管理",
		"Admins":     admins,
		"Paging":     paging,
		"AdminPage":  true,
		"ActiveMenu": "members",
	}
	c.Render(w, r, "admin/member_list", data)
}

// CreateForm 创建管理员表单页面
func (c *Member) CreateForm(w http.ResponseWriter, r *http.Request) {
	roles, _ := c.PermService.ListRoles()
	data := map[string]interface{}{
		"Title":      "创建管理员",
		"Action":     "/admin/members",
		"Method":     "POST",
		"FormAdmin":  model.Admin{Status: 1},
		"Roles":      roles,
		"AdminPage":  true,
		"ActiveMenu": "members",
	}
	c.Render(w, r, "admin/member_form", data)
}

// Create 创建管理员
func (c *Member) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	roleID, _ := strconv.ParseUint(r.FormValue("role_id"), 10, 64)
	req := &model.AdminCreateReq{
		Username: r.FormValue("username"),
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
		Nickname: r.FormValue("nickname"),
		RoleID:   uint(roleID),
	}

	newAdmin, err := c.Service.Create(req)
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "创建管理员失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:创建账号", "operator", operator.Username, "new_admin", newAdmin.Username)

	c.Redirect(w, r, "/admin/members")
}

// EditForm 编辑管理员表单页面
func (c *Member) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	admin, err := c.Service.GetByID(uint(id))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "管理员不存在")
		return
	}

	roles, _ := c.PermService.ListRoles()
	data := map[string]interface{}{
		"Title":      "编辑管理员",
		"Action":     "/admin/members/" + chi.URLParam(r, "id"),
		"Method":     "PUT",
		"FormAdmin":  admin,
		"Roles":      roles,
		"AdminPage":  true,
		"ActiveMenu": "members",
	}
	c.Render(w, r, "admin/member_form", data)
}

// Update 更新管理员
func (c *Member) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	status, _ := strconv.Atoi(r.FormValue("status"))
	roleID, _ := strconv.ParseUint(r.FormValue("role_id"), 10, 64)
	roleIDUint := uint(roleID)
	req := &model.AdminUpdateReq{
		Email:    r.FormValue("email"),
		Nickname: r.FormValue("nickname"),
		RoleID:   &roleIDUint,
		Status:   &status,
	}

	if _, err := c.Service.Update(uint(id), req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "更新管理员失败")
		return
	}

	c.Redirect(w, r, "/admin/members")
}

// Delete 删除管理员（不能删除自己）
func (c *Member) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	currentAdmin := middleware.GetCurrentAdmin(r)
	if currentAdmin != nil && currentAdmin.ID == uint(id) {
		common.JSONError(w, http.StatusForbidden, "不能删除当前登录账号")
		return
	}

	if err := c.Service.Delete(uint(id)); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "删除管理员失败")
		return
	}

	slog.Info("管理员操作:删除账号", "operator", currentAdmin.Username, "target_id", id)

	common.JSONSuccess(w, nil)
}

// APIList 管理员列表 API（JSON）
func (c *Member) APIList(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := config.Get().Pagination.DefaultPageSize

	admins, total, err := c.Service.List(page, pageSize)
	if err != nil {
		c.JSONError(w, http.StatusInternalServerError, "获取管理员列表失败")
		return
	}

	c.JSON(w, map[string]interface{}{
		"admins": admins,
		"total":  total,
		"page":   page,
	})
}
