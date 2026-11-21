package token

import "testing"

func TestIssueParseRoundTrip(t *testing.T) {
	const secret = "test-secret-0123456789abcdef-0123456789abcdef"
	tok, err := Issue(42, 3, secret)
	if err != nil {
		t.Fatalf("Issue() 出错: %v", err)
	}
	claims, err := Parse(tok, secret)
	if err != nil {
		t.Fatalf("Parse() 出错: %v", err)
	}
	if claims.UID != 42 || claims.TV != 3 {
		t.Fatalf("claims 不一致: got uid=%d tv=%d, want uid=42 tv=3", claims.UID, claims.TV)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	const secret = "test-secret-0123456789abcdef-0123456789abcdef"
	tok, err := Issue(1, 1, secret)
	if err != nil {
		t.Fatalf("Issue() 出错: %v", err)
	}
	if _, err := Parse(tok, "another-secret-0123456789abcdef-0123456789"); err == nil {
		t.Fatal("换密钥解析应当失败")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse("not-a-token", "whatever-secret-0123456789abcdef-01234567"); err == nil {
		t.Fatal("垃圾串解析应当失败")
	}
}
