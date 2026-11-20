package c2engine

import (
	"encoding/json"
	"os"
	"testing"
)

// 黄金向量：抓包得到的 9 条官方任务记录，必须能被复刻端的结构**逐字节**重现。
func TestOfficialTaskRoundTripMatchesCapture(t *testing.T) {
	data, err := os.ReadFile("../testdata/original/wire/task_records.json")
	if err != nil {
		t.Skipf("capture missing: %v", err)
	}
	var cap struct {
		Samples []struct {
			Raw string `json:"_raw"`
		} `json:"samples"`
	}
	if err := json.Unmarshal(data, &cap); err != nil {
		t.Fatalf("parse capture: %v", err)
	}
	if len(cap.Samples) == 0 {
		t.Skip("no samples")
	}
	for i, s := range cap.Samples {
		task, err := DecodeOfficialTask([]byte(s.Raw))
		if err != nil {
			t.Fatalf("sample %d: decode: %v", i, err)
		}
		got, err := task.Encode()
		if err != nil {
			t.Fatalf("sample %d: encode: %v", i, err)
		}
		if string(got) != s.Raw {
			t.Fatalf("sample %d: re-encoded bytes differ\n got: %s\nwant: %s", i, got, s.Raw)
		}
	}
}

// 构造器产出的记录必须与抓包里对应操作一致。
func TestNewOfficialTaskMatchesCapturedTypes(t *testing.T) {
	cases := []struct {
		typ  string
		z    []string
		want string
	}{
		{OfficialTaskExec, []string{"id"}, `{"ConnType":"v","Host":"v","LocalProxy":false,"RemoteAddr":"v","Req":{"Pass":"v","Type":"M","File":null,"Z1":"id","Z2":"","Z3":"","Z4":"","Z5":""},"Option":{"Timeout":5000000000}}`},
		{OfficialTaskListDir, []string{"/tmp"}, `{"ConnType":"v","Host":"v","LocalProxy":false,"RemoteAddr":"v","Req":{"Pass":"v","Type":"B","File":null,"Z1":"/tmp","Z2":"","Z3":"","Z4":"","Z5":""},"Option":{"Timeout":5000000000}}`},
		{OfficialTaskCat, []string{"/etc/hostname"}, `{"ConnType":"v","Host":"v","LocalProxy":false,"RemoteAddr":"v","Req":{"Pass":"v","Type":"C","File":null,"Z1":"/etc/hostname","Z2":"","Z3":"","Z4":"","Z5":""},"Option":{"Timeout":5000000000}}`},
	}
	for _, c := range cases {
		got, err := NewOfficialTask(c.typ, c.z...).Encode()
		if err != nil {
			t.Fatalf("%s: %v", c.typ, err)
		}
		if string(got) != c.want {
			t.Fatalf("%s mismatch:\n got: %s\nwant: %s", c.typ, got, c.want)
		}
	}
}

// 上传记录：File 字段是 base64 内容，Z1 是目标路径。
func TestOfficialTaskUploadFile(t *testing.T) {
	task := NewOfficialTask(OfficialTaskUpload, "/tmp/up_m1.txt").WithUploadFile([]byte("hello-m1\n"))
	got, err := task.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	task2, err := DecodeOfficialTask(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if task2.Req.File == nil || *task2.Req.File != "aGVsbG8tbTEK" {
		t.Fatalf("file field = %v", task2.Req.File)
	}
	if task2.Req.Z1 != "/tmp/up_m1.txt" {
		t.Fatalf("Z1 = %q", task2.Req.Z1)
	}
}

// 官方认证算法（R22 实证）：挑战 = md5(版本号)，应答 = md5(vkey)。
func TestOfficialAuthChallengeAndResponse(t *testing.T) {
	// md5("4.9.3") —— 抓包里服务端反复下发的常量
	if got := OfficialAuthChallenge("4.9.3"); got != "a89a6dc509abfd63bcbed973e249120e" {
		t.Fatalf("challenge = %s", got)
	}
	// 两次不同实例的应答（vkey 不同）
	if got := OfficialAuthResponse("VK-P"); got != "9e09b0be529f6a4206198ad9dc340124" {
		t.Fatalf("response(VK-P) = %s", got)
	}
	if got := OfficialAuthResponse("VK9"); got != "224edc5281ea0aaeece8382983dce811" {
		t.Fatalf("response(VK9) = %s", got)
	}
}
