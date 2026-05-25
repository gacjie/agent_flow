package common

const (
	// Cookie 名称
	SessionCookieName = "session_token"
	CSRFCookieName    = "_csrf_token"
	CSRFFieldName     = "_csrf"

	// Token 长度（字节数，hex 编码后长度翻倍）
	CSRFTokenLen    = 16
	SessionTokenLen = 32
)
