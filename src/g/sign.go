package g

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 节点间 API 安全:
// 1. HMAC-SHA256 请求签名(基于集群接入令牌), 时间戳±300s + nonce 防重放
// 2. 配置同步内容 AES-256-GCM 加密(密钥=SHA256(令牌))
// 开关: Base["Apisign"]=1 时节点间接口强制验签(IP互信不再放行)

const signWindow = 300 // 秒

var (
	nonceMu   sync.Mutex
	nonceSeen = map[string]int64{} // nonce -> 过期时间(unix)
)

func clusterKey(token string) []byte {
	sum := sha256.Sum256([]byte("zenlenet-pingmesh:" + token))
	return sum[:]
}

func signOf(token, path, ts, nonce string) string {
	mac := hmac.New(sha256.New, clusterKey(token))
	mac.Write([]byte(path + "|" + ts + "|" + nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

func newNonce() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SignURL 为节点间请求追加签名参数(ts/nonce/sign)
func SignURL(raw, token string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := newNonce()
	q := u.Query()
	q.Set("ts", ts)
	q.Set("nonce", nonce)
	q.Set("sign", signOf(token, u.Path, ts, nonce))
	u.RawQuery = q.Encode()
	return u.String()
}

// SignFormFields 为 POST 表单生成签名字段
func SignFormFields(path, token string) map[string]string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := newNonce()
	return map[string]string{
		"ts":    ts,
		"nonce": nonce,
		"sign":  signOf(token, path, ts, nonce),
	}
}

// VerifySign 校验请求签名(query 或 form 中的 ts/nonce/sign)
func VerifySign(path, ts, nonce, sign string) bool {
	if ts == "" || nonce == "" || sign == "" {
		return false
	}
	tsv, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	now := time.Now().Unix()
	if tsv < now-signWindow || tsv > now+signWindow {
		return false
	}
	expect := signOf(Cfg.Password, path, ts, nonce)
	if !hmac.Equal([]byte(expect), []byte(strings.ToLower(sign))) {
		return false
	}
	// nonce 防重放
	nonceMu.Lock()
	defer nonceMu.Unlock()
	if exp, ok := nonceSeen[nonce]; ok && exp > now {
		return false
	}
	// 顺手清理过期 nonce
	if len(nonceSeen) > 10000 {
		for k, exp := range nonceSeen {
			if exp <= now {
				delete(nonceSeen, k)
			}
		}
	}
	nonceSeen[nonce] = now + signWindow*2
	return true
}

// SignRequired 是否强制节点间验签
func SignRequired() bool {
	return Cfg.Base != nil && Cfg.Base["Apisign"] == 1
}

/* ---------- 配置同步加密 ---------- */

const encPrefix = "ENC1:"

// EncryptPayload AES-256-GCM 加密(输出 ENC1:base64(nonce+ciphertext))
func EncryptPayload(data []byte, token string) (string, error) {
	block, err := aes.NewCipher(clusterKey(token))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, data, nil)
	return encPrefix + base64.StdEncoding.EncodeToString(append(nonce, ct...)), nil
}

// IsEncryptedPayload 判断响应是否为加密载荷
func IsEncryptedPayload(body []byte) bool {
	return strings.HasPrefix(strings.TrimSpace(string(body)), encPrefix)
}

// DecryptPayload 解密 ENC1: 载荷
func DecryptPayload(body []byte, token string) ([]byte, error) {
	s := strings.TrimSpace(string(body))
	if !strings.HasPrefix(s, encPrefix) {
		return nil, errors.New("not an encrypted payload")
	}
	raw, err := base64.StdEncoding.DecodeString(s[len(encPrefix):])
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(clusterKey(token))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("payload too short")
	}
	return gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
}
