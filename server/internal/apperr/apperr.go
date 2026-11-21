// Package apperr 领域错误哨兵：各层共用的「错误语言」。
// 有了它，handler/middleware 只认错误语义、不依赖 service 具体实现，
// 即可各自映射 HTTP 状态码，避免业务层与传输层横向耦合。
package apperr

import "errors"

var (
	ErrInvalidInput       = errors.New("参数不合法")
	ErrInvalidEmail       = errors.New("邮箱格式不正确")
	ErrUsernameTaken      = errors.New("用户名已存在")
	ErrEmailTaken         = errors.New("邮箱已被使用")
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrInvalidToken       = errors.New("未登录或登录已过期")
	ErrBanned             = errors.New("账号封禁中")

	// 服务实例域
	ErrNameTaken        = errors.New("服务名称已存在")
	ErrInstanceNotFound = errors.New("服务实例不存在")
	ErrBadBaseURL       = errors.New("Base URL 不合法：须为 http/https 且含主机名")
	ErrAuthType         = errors.New("鉴权方式仅支持 none 或 bearer")
)
