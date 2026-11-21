package crypto

import "testing"

const jwtSecret = "test-jwt-secret-0123456789abcdef-0123456789"

func TestSealOpenRoundTrip(t *testing.T) {
	b := NewBox("", jwtSecret)
	sealed, err := b.Seal("bearer-token-plaintext")
	if err != nil {
		t.Fatalf("Seal() 出错: %v", err)
	}
	if sealed == "bearer-token-plaintext" {
		t.Fatal("密文不应与明文相同")
	}
	plain, err := b.Open(sealed)
	if err != nil {
		t.Fatalf("Open() 出错: %v", err)
	}
	if plain != "bearer-token-plaintext" {
		t.Fatalf("往返不一致: got %q", plain)
	}
}

func TestOpenRejectsGarbage(t *testing.T) {
	b := NewBox("", jwtSecret)
	for _, bad := range []string{"", "not base64!!!", "YWJj"} { // "YWJj" 解码后长度 < 24
		if _, err := b.Open(bad); err == nil {
			t.Fatalf("Open(%q) 应当失败", bad)
		}
	}
}

func TestDerivedKeyStableAndIsolated(t *testing.T) {
	// 同一 JWT_SECRET 派生的两个 Box 可互通（密文可解）
	a := NewBox("", jwtSecret)
	b := NewBox("", jwtSecret)
	sealed, err := a.Seal("hello")
	if err != nil {
		t.Fatalf("Seal() 出错: %v", err)
	}
	if plain, err := b.Open(sealed); err != nil || plain != "hello" {
		t.Fatalf("同派生密钥应互通: plain=%q err=%v", plain, err)
	}
	// 不同 JWT_SECRET 派生 → 不可解（密钥隔离生效）
	c := NewBox("", jwtSecret+"-other")
	if _, err := c.Open(sealed); err == nil {
		t.Fatal("不同派生密钥解密应当失败")
	}
	// 显式 CRED_SECRET 覆盖派生口径
	d := NewBox("explicit-cred-secret", jwtSecret)
	sealed2, err := d.Seal("x")
	if err != nil {
		t.Fatalf("Seal() 出错: %v", err)
	}
	if _, err := a.Open(sealed2); err == nil {
		t.Fatal("显式密钥与派生密钥应当隔离")
	}
}
