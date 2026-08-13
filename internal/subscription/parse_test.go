package subscription

import "testing"

func TestParse_MixedValidAndInvalid(t *testing.T) {
	input := []byte(`vless://11111111-2222-3333-4444-555555555555@example.com:443?type=tcp&security=none#primary
vmess://someBase64Blob#should-be-skipped
vless://22222222-3333-4444-5555-666666666666@example.org:8443?type=ws&security=tls&sni=cdn.example.org#backup

not-a-uri-at-all
vless://@bad:443?type=tcp&security=none#missing-uuid`)

	profiles, warnings := Parse(input)

	if len(profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2; profiles=%#v warnings=%v", len(profiles), profiles, warnings)
	}
	if profiles[0].Remark != "primary" || profiles[1].Remark != "backup" {
		t.Errorf("profiles = %#v, want remarks [primary backup]", profiles)
	}
	if len(warnings) != 3 {
		t.Errorf("len(warnings) = %d, want 3 (vmess line, non-uri line, missing-uuid line); got %v", len(warnings), warnings)
	}
}

func TestParse_Empty(t *testing.T) {
	profiles, warnings := Parse([]byte(""))
	if len(profiles) != 0 || len(warnings) != 0 {
		t.Errorf("Parse(empty) = (%v, %v), want (nil, nil)", profiles, warnings)
	}
}

func TestParse_BlankLinesIgnored(t *testing.T) {
	input := []byte("\n\n   \nvless://11111111-2222-3333-4444-555555555555@example.com:443?type=tcp&security=none#a\n\n")
	profiles, warnings := Parse(input)
	if len(profiles) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(profiles))
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
}
