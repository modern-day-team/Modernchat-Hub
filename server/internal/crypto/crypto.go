// Package crypto 服务凭据加解密：nacl/secretbox。
// 密钥用独立前缀 mc-hub-cred-v1: 派生，不与主仓库的 mc-settings-v1 口径混用——
// 项目组服务共享 JWT_SECRET 时，Hub 的凭据语义必须隔离（见 AGENTS.md 密钥警告）。
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/secretbox"
)

// derivePrefix Hub 自己的派生前缀：密钥语义与主仓库隔离。
const derivePrefix = "mc-hub-cred-v1:"

// ErrBadCiphertext 密文不合法（解码失败、长度不足或解密失败）。
var ErrBadCiphertext = errors.New("密文不合法")

// Box 凭据加解密器。
type Box struct{ key [32]byte }

// NewBox 构造：CRED_SECRET 未配置时由 JWT_SECRET 加独立前缀派生（两口径都 sha256 归一到 32 字节）。
func NewBox(credSecret, jwtSecret string) *Box {
	material := credSecret
	if material == "" {
		material = derivePrefix + jwtSecret
	}
	return &Box{key: sha256.Sum256([]byte(material))}
}

// Seal 加密为 base64(nonce||ciphertext)。
func (b *Box) Seal(plaintext string) (string, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}
	sealed := secretbox.Seal(nonce[:], []byte(plaintext), &nonce, &b.key)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open 解密 base64(nonce||ciphertext)。
func (b *Box) Open(sealed string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(data) < 24 {
		return "", ErrBadCiphertext
	}
	var nonce [24]byte
	copy(nonce[:], data[:24])
	plain, ok := secretbox.Open(nil, data[24:], &nonce, &b.key)
	if !ok {
		return "", ErrBadCiphertext
	}
	return string(plain), nil
}
