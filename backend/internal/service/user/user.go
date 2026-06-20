package user

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/pkg/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	db     *gorm.DB
	config *config.Config
}

func NewUserService(db *gorm.DB, cfg *config.Config) *UserService {
	return &UserService{
		db:     db,
		config: cfg,
	}
}

// Register 用户注册
func (s *UserService) Register(req *RegisterRequest) (*UserResponse, error) {
	// 检查用户名是否已注册
	var existingUser model.User
	if err := s.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		return nil, errors.New("用户名已存在")
	}

	// 验证用户名长度
	if len(req.Username) < 3 {
		return nil, errors.New("用户名至少3个字符")
	}

	// 验证密码长度
	if len(req.Password) < 6 {
		return nil, errors.New("密码至少6位")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 创建用户
	nn := req.Nickname
	if nn == "" {
		nn = req.Username
	}
	user := &model.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Nickname: nn,
		Email:    req.Email, // 可选
		Status:   1,
	}
	// 邮箱唯一性软校验（空值忽略）
	if req.Email != "" {
		var dup model.User
		if err := s.db.Where("email = ?", req.Email).First(&dup).Error; err == nil {
			return nil, errors.New("该邮箱已注册")
		}
	}

	// 创建积分账户
	credits := &model.UserCredits{
		Balance:        s.config.RegisterCredits,
		TotalRecharged: s.config.RegisterCredits,
	}

	// 事务处理
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		credits.UserID = user.ID
		if err := tx.Create(credits).Error; err != nil {
			return err
		}

		// 写入积分流水
		tx.Create(&model.CreditTransaction{
			UserID:      user.ID,
			Type:        "gift",
			Amount:      s.config.RegisterCredits,
			Balance:     s.config.RegisterCredits,
			Source:      "register",
			Description: "注册赠送积分",
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 生成JWT
	token, err := s.GenerateToken(user.ID.String())
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:       user.ID.String(),
		Username: user.Username,
		Nickname: user.Nickname,
		Token:    token,
	}, nil
}

// Login 用户登录（支持用户名 / 邮箱 / 手机号 任一）
func (s *UserService) Login(req *LoginRequest) (*UserResponse, error) {
	var user model.User
	id := req.Username
	// 如果含 @，按 email 查；否则 username OR phone
	if strings.Contains(id, "@") {
		if err := s.db.Where("email = ?", id).First(&user).Error; err != nil {
			return nil, errors.New("用户不存在")
		}
	} else {
		if err := s.db.Where("username = ? OR phone = ?", id, id).First(&user).Error; err != nil {
			return nil, errors.New("用户不存在")
		}
	}

	if user.Status != 1 {
		return nil, errors.New("账号已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("密码错误")
	}

	token, err := s.GenerateToken(user.ID.String())
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:       user.ID.String(),
		Username: user.Username,
		Nickname: user.Nickname,
		Token:    token,
	}, nil
}

// GenerateToken 生成JWT
func (s *UserService) GenerateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Duration(s.config.JWTExpire) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// ParseToken 解析JWT
func (s *UserService) ParseToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, ok := claims["user_id"].(string)
		if !ok {
			return "", errors.New("invalid token")
		}
		return userID, nil
	}

	return "", errors.New("invalid token")
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(userID uuid.UUID) (*model.User, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetCredits 获取用户积分
func (s *UserService) GetCredits(userID uuid.UUID) (*model.UserCredits, error) {
	var credits model.UserCredits
	if err := s.db.Where("user_id = ?", userID).First(&credits).Error; err != nil {
		return nil, err
	}
	return &credits, nil
}

// 请求和响应结构
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email"` // 可选；建议填写，用于找回密码
	Nickname string `json:"nickname"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Token    string `json:"token"`
}


// isValidEmail 简单邮箱校验
func isValidEmail(s string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(s)
}

func (s *UserService) ChangePassword(userID uuid.UUID, oldPwd, newPwd string) error {
	if len(newPwd) < 6 {
		return errors.New("新密码至少6位")
	}
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return errors.New("用户不存在")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd)); err != nil {
		return errors.New("原密码错误")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(&user).Update("password", string(hashed)).Error
}

// ===== 第三方登录占位 =====
// LoginByOAuth 处理微信/Apple 等第三方登录。
// 当前为占位实现：真实集成时需要：
//   - provider=wechat: 用 OAuth code 换 access_token + userinfo (openid, unionid, nickname, headimgurl)
//   - provider=apple: 验证 identity_token (JWT, 苹果公钥) 拿 sub/email
// 然后按 openid/sub 查 user_oauth 表，没记录就 upsert 一条新 user（自动注册）。
func (s *UserService) LoginByOAuth(provider, code, nickname, avatar string) (*UserResponse, error) {
	if provider == "" {
		return nil, errors.New("provider 必填")
	}
	if code == "" {
		return nil, errors.New("授权码不能为空")
	}
	// TODO: 这里调用 provider 真实接口拿 openID/sub
	openID := provider + "_demo_" + code[:min(len(code), 8)]

	var user model.User
	err := s.db.Where("username = ?", openID).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		randomPwd := fmt.Sprintf("oauth_%s", uuid.New().String())
		hashed, _ := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)
		nn := nickname
		if nn == "" {
			nn = provider + "用户"
		}
		user = model.User{
			Username: openID,
			Password: string(hashed),
			Nickname: nn,
			Avatar:   avatar,
			Status:   1,
		}
		if cerr := s.db.Create(&user).Error; cerr != nil {
			return nil, cerr
		}
		// 赠送积分
		s.db.Create(&model.UserCredits{UserID: user.ID, Balance: 10})
	} else if err != nil {
		return nil, err
	}

	if user.Status != 1 {
		return nil, errors.New("账号已被禁用")
	}
	token, err := s.GenerateToken(user.ID.String())
	if err != nil {
		return nil, err
	}
	return &UserResponse{
		ID:       user.ID.String(),
		Username: user.Username,
		Nickname: user.Nickname,
		Token:    token,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


// ===== 密码重置（基于邮箱，无邮件服务 → 日志打印重置链接） =====
//
// 流程：
//   1. 用户 POST /auth/forgot-password {email}
//   2. 后端查找该邮箱对应的 user，生成一次性 token（30 分钟有效）
//   3. 当前无 SMTP：把 "重置链接" 用 log.Printf 打到控制台（开发可转交用户）
//      真实部署时应改为发送邮件
//   4. 用户 POST /auth/reset-password {token, new_password} 完成重置

// resetTokenTTL 密码重置 token 有效期
const resetTokenTTL = 30 * time.Minute

// RequestPasswordReset 申请密码重置（按邮箱）
// 返回 (token, nil) 给前端用于 demo 显示；生产不应回传
func (s *UserService) RequestPasswordReset(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", errors.New("请输入邮箱")
	}
	if !isValidEmail(email) {
		return "", errors.New("邮箱格式不正确")
	}
	var user model.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		// 出于安全考虑：无论邮箱是否注册，都返回 ok
		// 但为了让 demo 可用，这里给一个明确错误提示
		return "", errors.New("该邮箱未注册")
	}
	// 生成 32 字节 token
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)

	entry := &model.PasswordResetToken{
		UserID:    user.ID,
		Email:     email,
		Token:     token,
		ExpiresAt: time.Now().Add(resetTokenTTL),
		Used:      false,
	}
	if err := s.db.Create(entry).Error; err != nil {
		return "", err
	}

	// 构造"重置链接"并打印到日志（替代邮件）
	resetPath := fmt.Sprintf("/reset-password?token=%s", token)
	log.Printf("[PasswordReset] email=%s username=%s token=%s link=%s",
		email, user.Username, token, resetPath)

	return token, nil
}

// ConfirmPasswordReset 用 token 完成密码重置
func (s *UserService) ConfirmPasswordReset(token, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("新密码至少6位")
	}
	var entry model.PasswordResetToken
	err := s.db.Where("token = ?", token).First(&entry).Error
	if err != nil {
		return errors.New("重置链接无效")
	}
	if entry.Used {
		return errors.New("重置链接已被使用")
	}
	if time.Now().After(entry.ExpiresAt) {
		return errors.New("重置链接已过期，请重新申请")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", entry.UserID).Update("password", string(hashed)).Error; err != nil {
			return err
		}
		return tx.Model(&entry).Update("used", true).Error
	})
	return err
}
type codeEntry struct {
	code    string
	expire  time.Time
	purpose string
}

var (
	codeMu    sync.RWMutex
	codeStore = make(map[string]codeEntry)
	codeTTL   = 5 * time.Minute
)

func newCode() string {
	// 用 crypto/rand 生成 0-999999 之间的数字
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "888888"
	}
	n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1000000
	return fmt.Sprintf("%06d", n)
}

func (s *UserService) SendCode(phone, purpose string) (string, error) {
	if len(phone) < 3 {
		return "", errors.New("手机号格式错误")
	}
	if purpose == "" {
		purpose = "login"
	}
	codeMu.Lock()
	defer codeMu.Unlock()
	if e, ok := codeStore[phone+":"+purpose]; ok && time.Until(e.expire) > 4*time.Minute+50*time.Second {
		return "", errors.New("请求过于频繁，请稍后再试")
	}
	code := newCode()
	codeStore[phone+":"+purpose] = codeEntry{code: code, expire: time.Now().Add(codeTTL), purpose: purpose}
	return code, nil
}

func (s *UserService) LoginOrRegisterByCode(phone, code, nickname string) (*UserResponse, error) {
	codeMu.RLock()
	e, ok := codeStore[phone+":login"]
	codeMu.RUnlock()
	if !ok || e.code != code || time.Now().After(e.expire) {
		return nil, errors.New("验证码错误或已过期")
	}
	codeMu.Lock()
	delete(codeStore, phone+":login")
	codeMu.Unlock()

	var user model.User
	err := s.db.Where("username = ?", phone).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		randomPwd := fmt.Sprintf("auto_%s", uuid.New().String())
		hashed, herr := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)
		if herr != nil {
			return nil, herr
		}
		nn := nickname
		if nn == "" {
			nn = phone
		}
		user = model.User{Username: phone, Password: string(hashed), Nickname: nn, Status: 1}
		if cerr := s.db.Create(&user).Error; cerr != nil {
			return nil, cerr
		}
		s.db.Create(&model.UserCredits{UserID: user.ID, Balance: 10})
	} else if err != nil {
		return nil, err
	}
	if user.Status != 1 {
		return nil, errors.New("账号已被禁用")
	}
	token, err := s.GenerateToken(user.ID.String())
	if err != nil {
		return nil, err
	}
	return &UserResponse{
		ID:       user.ID.String(),
		Username: user.Username,
		Nickname: user.Nickname,
		Token:    token,
	}, nil
}
