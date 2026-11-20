package c2engine

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// wireSample 是抓包样本的结构（见 testdata/original/wire/tcp_version_handshake.json）。
type wireSample struct {
	Samples []struct {
		Name string `json:"name"`
		Hex  string `json:"hex"`
	} `json:"samples"`
	DecryptedPlainHex string `json:"decrypted_plaintext_hex"`
}

// 黄金向量：原版 agent 的握手帧必须能被复刻端的 FrameDecrypt 解开，
// 且明文与抓包记录一致。这证明**帧层**（长度前缀 + nonce + GCM）与密钥派生
// （key = hex(md5(salt))）与原版完全一致。
func TestFrameDecryptOfficialAgentHandshake(t *testing.T) {
	data, err := os.ReadFile("../testdata/original/wire/tcp_version_handshake.json")
	if err != nil {
		t.Skipf("wire sample missing: %v", err)
	}
	var s wireSample
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parse sample: %v", err)
	}
	if len(s.Samples) == 0 {
		t.Fatal("no samples")
	}
	wantPlain, _ := hex.DecodeString(s.DecryptedPlainHex)
	if len(wantPlain) == 0 {
		t.Fatal("missing expected plaintext")
	}
	for _, smp := range s.Samples {
		wire, err := hex.DecodeString(smp.Hex)
		if err != nil {
			t.Fatalf("%s: bad hex: %v", smp.Name, err)
		}
		// FrameDecrypt 接受**完整线格式**（含 4 字节长度前缀）
		plain, err := FrameDecrypt(wire, "")
		if err != nil {
			t.Fatalf("%s: FrameDecrypt: %v", smp.Name, err)
		}
		if string(plain) != string(wantPlain) {
			t.Fatalf("%s: plaintext = %x, want %x", smp.Name, plain, wantPlain)
		}
	}
}

// 反向：复刻端加密出的帧必须能被自己解开（自洽性），并且长度前缀正确。
func TestFrameEncryptRoundTripWithLengthPrefix(t *testing.T) {
	plain := []byte("\x05\x00\x00\x004.9.3")
	body, err := FrameEncrypt(plain, "")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// 线格式 = [u32 LE 长度][12B nonce][密文 len][16B tag]
	if len(body) != 4+12+len(plain)+16 {
		t.Fatalf("frame size = %d, want %d", len(body), 4+12+len(plain)+16)
	}
	got, err := FrameDecrypt(body, "")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("round trip = %x", got)
	}
}
