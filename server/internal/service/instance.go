package service

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/crypto"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// DefaultHealthPath 未指定健康路径时的默认值（与主仓库一致）。
const DefaultHealthPath = "/api/health"

// AuthNone / AuthBearer 支持的鉴权方式。
const (
	AuthNone   = "none"
	AuthBearer = "bearer"
)

// instanceRepo 消费方定义的窄接口（由 store.InstanceStore 实现），
// 单测可用假仓储驱动全部业务分支而不依赖 MySQL。
type instanceRepo interface {
	List() ([]model.ServiceInstance, error)
	FindByID(id uint) (*model.ServiceInstance, error)
	FindByName(name string) (*model.ServiceInstance, error)
	Create(i *model.ServiceInstance) error
	Save(i *model.ServiceInstance) error
	Delete(id uint) error
	SaveProbeResult(id uint, status string, version *string, checkedAt time.Time) error
}

// InstanceService 服务实例业务：注册/编辑/探测的规则唯一归属地。
type InstanceService struct {
	repo instanceRepo
	box  *crypto.Box
}

// NewInstanceService 构造。
func NewInstanceService(repo instanceRepo, box *crypto.Box) *InstanceService {
	return &InstanceService{repo: repo, box: box}
}

// validate 校验并归一（名称去空格、BaseURL 可解析、健康路径带前导斜杠）。
func validate(name, baseURL, authType, healthPath string) (string, string, string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 30 {
		return "", "", "", "", apperr.ErrInvalidInput
	}
	baseURL = strings.TrimSpace(baseURL)
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", "", "", apperr.ErrBadBaseURL
	}
	if authType != AuthNone && authType != AuthBearer {
		return "", "", "", "", apperr.ErrAuthType
	}
	healthPath = strings.TrimSpace(healthPath)
	if healthPath == "" {
		healthPath = DefaultHealthPath
	}
	if !strings.HasPrefix(healthPath, "/") {
		healthPath = "/" + healthPath
	}
	return name, baseURL, authType, healthPath, nil
}

// Create 注册服务实例：唯一性、URL/鉴权校验，bearer 凭据加密落库。
func (s *InstanceService) Create(name, baseURL, authType, token, healthPath string, enabled bool, note string) (*model.ServiceInstance, error) {
	name, baseURL, authType, healthPath, err := validate(name, baseURL, authType, healthPath)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.FindByName(name); err == nil {
		return nil, apperr.ErrNameTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var cipher string
	if authType == AuthBearer {
		if token == "" {
			return nil, apperr.ErrInvalidInput
		}
		if cipher, err = s.box.Seal(token); err != nil {
			return nil, err
		}
	}
	inst := &model.ServiceInstance{
		Name:            name,
		BaseURL:         baseURL,
		AuthType:        authType,
		TokenCiphertext: cipher,
		HealthPath:      healthPath,
		Enabled:         enabled,
		Note:            strings.TrimSpace(note),
		LastStatus:      "unknown",
	}
	if err := s.repo.Create(inst); err != nil {
		// 名称唯一索引兜底并发场景
		return nil, err
	}
	inst.FillHasToken()
	return inst, nil
}

// Update 编辑实例：token 传空表示保持原凭据不变。
func (s *InstanceService) Update(id uint, name, baseURL, authType, token, healthPath string, enabled bool, note string) (*model.ServiceInstance, error) {
	inst, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.ErrInstanceNotFound
		}
		return nil, err
	}
	name, baseURL, authType, healthPath, err = validate(name, baseURL, authType, healthPath)
	if err != nil {
		return nil, err
	}
	if other, err := s.repo.FindByName(name); err == nil && other.ID != inst.ID {
		return nil, apperr.ErrNameTaken
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	inst.Name = name
	inst.BaseURL = baseURL
	inst.AuthType = authType
	inst.HealthPath = healthPath
	inst.Enabled = enabled
	inst.Note = strings.TrimSpace(note)
	if token != "" {
		cipher, err := s.box.Seal(token)
		if err != nil {
			return nil, err
		}
		inst.TokenCiphertext = cipher
	}
	if err := s.repo.Save(inst); err != nil {
		return nil, err
	}
	inst.FillHasToken()
	return inst, nil
}

// List 全量列表。
func (s *InstanceService) List() ([]model.ServiceInstance, error) {
	list, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	model.FillHasTokenAll(list)
	return list, nil
}

// Get 单个实例。
func (s *InstanceService) Get(id uint) (*model.ServiceInstance, error) {
	inst, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.ErrInstanceNotFound
		}
		return nil, err
	}
	inst.FillHasToken()
	return inst, nil
}

// Delete 删除实例。
func (s *InstanceService) Delete(id uint) error {
	return s.repo.Delete(id)
}

// Probe 立即探测一次：健康路径 2xx → lastStatus=ok，否则 fail；
// 版本端点（BaseURL + /api/version）可选，取到则记录 LastVersion。
// 探测结果是数据不是异常：网络失败只写回 fail，不向上抛错。
func (s *InstanceService) Probe(id uint) (*model.ServiceInstance, error) {
	inst, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.ErrInstanceNotFound
		}
		return nil, err
	}
	status, version := s.probeOne(inst)
	if err := s.repo.SaveProbeResult(inst.ID, status, version, time.Now()); err != nil {
		return nil, err
	}
	inst.LastStatus = status
	inst.LastVersion = version
	now := time.Now()
	inst.LastCheckAt = &now
	inst.FillHasToken()
	return inst, nil
}

// probeHTTP 探测用客户端（包级变量便于测试替换目标地址；超时 5s）。
var probeHTTP = &http.Client{Timeout: 5 * time.Second}

func (s *InstanceService) probeOne(inst *model.ServiceInstance) (string, *string) {
	base := strings.TrimRight(inst.BaseURL, "/")
	if !s.getOK(base+inst.HealthPath, inst.AuthType, inst.TokenCiphertext) {
		// 日志只记实例名与 URL，不记凭据
		slog.Warn("服务探测失败", "instance", inst.Name, "url", base+inst.HealthPath)
		return "fail", nil
	}
	return "ok", s.fetchVersion(base, inst.AuthType, inst.TokenCiphertext)
}

func (s *InstanceService) getOK(rawURL, authType, cipher string) bool {
	resp, err := s.doGet(rawURL, authType, cipher)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// fetchVersion 版本信息可选：端点缺失/非 2xx/解析失败一律返回 nil。
func (s *InstanceService) fetchVersion(base, authType, cipher string) *string {
	resp, err := s.doGet(base+"/api/version", authType, cipher)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	var vi struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vi); err != nil || vi.Version == "" {
		return nil
	}
	return &vi.Version
}

func (s *InstanceService) doGet(rawURL, authType, cipher string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if authType == AuthBearer && cipher != "" {
		plain, err := s.box.Open(cipher)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+plain)
	}
	return probeHTTP.Do(req)
}
