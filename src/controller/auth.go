package controller

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/middleware"
	"agent_flow/src/service"
)

// Auth 登录/登出控制器
type Auth struct {
	Base
	AuthService    *service.AuthService
	CaptchaService *service.CaptchaService
}

// LoginForm 渲染登录页面
func (c *Auth) LoginForm(w http.ResponseWriter, r *http.Request) {
	// 已登录则跳转首页
	if middleware.GetCurrentAdmin(r) != nil {
		c.Redirect(w, r, "/admin")
		return
	}

	captchaID, captchaSVG := c.CaptchaService.Generate()
	data := map[string]interface{}{
		"Title":      "登录",
		"Error":      "",
		"CSRFToken":  middleware.GetCSRFTokenFromRequest(r),
		"CaptchaID":  captchaID,
		"CaptchaSVG": template.HTML(captchaSVG),
	}
	c.Render(w, r, "auth/login", data)
}

// Login 处理登录表单提交
func (c *Auth) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.renderLoginError(w, r, "表单解析失败")
		return
	}

	// 验证码校验
	captchaID := r.FormValue("captcha_id")
	captchaAnswer, _ := strconv.Atoi(r.FormValue("captcha"))
	if !c.CaptchaService.Verify(captchaID, captchaAnswer) {
		c.renderLoginError(w, r, "验证码错误")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		c.renderLoginError(w, r, "用户名和密码不能为空")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	token, err := c.AuthService.Login(username, password, ip, ua)
	if err != nil {
		msg := "用户名或密码错误"
		if appErr, ok := err.(*common.AppError); ok {
			msg = appErr.Message
		}
		c.renderLoginError(w, r, msg)
		return
	}

	// 设置 Session Cookie
	cfg := config.Get()
	http.SetCookie(w, &http.Cookie{
		Name:     common.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   cfg.Auth.SessionMaxAge,
		HttpOnly: cfg.Auth.CookieHTTPOnly,
		Secure:   cfg.Auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	// 登录成功后刷新 CSRF Cookie
	newCSRF, _ := common.GenerateToken(common.CSRFTokenLen)
	http.SetCookie(w, &http.Cookie{
		Name:     common.CSRFCookieName,
		Value:    newCSRF,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
	})

	// 首次登录引导修改密码
	admin, _ := c.AuthService.ValidateSession(token, ip)
	if admin != nil && admin.MustChangePassword {
		c.Redirect(w, r, "/admin/password")
	} else {
		c.Redirect(w, r, "/admin")
	}
}

// CaptchaRefresh 返回新验证码 JSON（供前端刷新）
func (c *Auth) CaptchaRefresh(w http.ResponseWriter, r *http.Request) {
	common.JSONSuccess(w, c.CaptchaService.GenerateJSON())
}

// Logout 登出
func (c *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(common.SessionCookieName)
	if err == nil && cookie.Value != "" {
		c.AuthService.Logout(cookie.Value)
	}

	// 清除 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     common.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})

	c.Redirect(w, r, "/auth/login")
}

// renderLoginError 渲染登录页并显示错误（自动刷新验证码）
func (c *Auth) renderLoginError(w http.ResponseWriter, r *http.Request, msg string) {
	captchaID, captchaSVG := c.CaptchaService.Generate()
	data := map[string]interface{}{
		"Title":      "登录",
		"Error":      msg,
		"CSRFToken":  middleware.GetCSRFTokenFromRequest(r),
		"CaptchaID":  captchaID,
		"CaptchaSVG": template.HTML(captchaSVG),
	}
	w.WriteHeader(http.StatusUnauthorized)
	c.Render(w, r, "auth/login", data)
}

// APILogin APP 专用 JSON 登录接口，返回 API Token
func (c *Auth) APILogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := common.BindJSON(r, &req); err != nil || req.Username == "" || req.Password == "" {
		common.JSONError(w, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}
	admin, err := c.AuthService.LoginForAPI(req.Username, req.Password, r.RemoteAddr)
	if err != nil {
		msg := "用户名或密码错误"
		if appErr, ok := err.(*common.AppError); ok {
			msg = appErr.Message
		}
		common.JSONError(w, http.StatusUnauthorized, msg)
		return
	}
	token, err := c.AuthService.CreateAPIToken(admin.ID, "mobile-app")
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "Token 创建失败")
		return
	}
	common.JSONSuccess(w, map[string]interface{}{
		"token":      token,
		"admin_name": admin.Nickname,
		"expires_at": time.Now().Add(90 * 24 * time.Hour),
	})
}

// APILogout APP 专用登出接口，吊销 API Token
func (c *Auth) APILogout(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 {
		c.AuthService.RevokeAPIToken(auth[7:])
	}
	common.JSONSuccess(w, nil)
}
