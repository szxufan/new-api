package common

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	"github.com/samber/lo"
)

var (
	maskURLPattern = regexp.MustCompile(`(http|https)://[^\s/$.?#].[^\s]*`)
	maskIPPattern  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// maskApiKeyPattern matches patterns like 'api_key:xxx' or "api_key:xxx" to mask the API key value
	maskApiKeyPattern = regexp.MustCompile(`(['"]?)api_key:([^\s'"]+)(['"]?)`)
)

func GetStringIfEmpty(str string, defaultValue string) string {
	if str == "" {
		return defaultValue
	}
	return str
}

func GetRandomString(length int) string {
	if length <= 0 {
		return ""
	}
	return lo.RandomString(length, lo.AlphanumericCharset)
}

// MapToJsonStr 将 map 序列化为 JSON 字符串。
// 序列化失败时返回 "{}" 而不是空字符串：空字符串不是合法 JSON，
// 会导致 PostgreSQL 端对 other 字段执行 ::jsonb 转换时报 SQLSTATE 22P02。
func MapToJsonStr(m map[string]interface{}) string {
	bytes, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func StrToMap(str string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	err := Unmarshal([]byte(str), &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func StrToJsonArray(str string) ([]interface{}, error) {
	var js []interface{}
	err := json.Unmarshal([]byte(str), &js)
	if err != nil {
		return nil, err
	}
	return js, nil
}

func IsJsonArray(str string) bool {
	var js []interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

func IsJsonObject(str string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

func String2Int(str string) int {
	num, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return num
}

func StringsContains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}

// StringToByteSlice []byte only read, panic on append
func StringToByteSlice(s string) []byte {
	tmp1 := (*[2]uintptr)(unsafe.Pointer(&s))
	tmp2 := [3]uintptr{tmp1[0], tmp1[1], tmp1[1]}
	return *(*[]byte)(unsafe.Pointer(&tmp2))
}

func EncodeBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func GetJsonString(data any) string {
	if data == nil {
		return ""
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// NormalizeBillingPreference clamps the billing preference to valid values.
func NormalizeBillingPreference(pref string) string {
	switch strings.TrimSpace(pref) {
	case "subscription_first", "wallet_first", "subscription_only", "wallet_only":
		return strings.TrimSpace(pref)
	default:
		return "subscription_first"
	}
}

// MaskEmail masks a user email to prevent PII leakage in logs
// Returns "***masked***" if email is empty, otherwise shows only the domain part
func MaskEmail(email string) string {
	if email == "" {
		return "***masked***"
	}

	// Find the @ symbol
	atIndex := strings.Index(email, "@")
	if atIndex == -1 {
		// No @ symbol found, return masked
		return "***masked***"
	}

	// Return only the domain part with @ symbol
	return "***@" + email[atIndex+1:]
}

// sensitiveQueryParamKeywords lists query parameter name keywords that carry secrets.
// Matching is case-insensitive substring based, so variants like access_key,
// client_secret or AUTH_TOKEN are also covered.
var sensitiveQueryParamKeywords = []string{
	"key", "secret", "token", "password", "passwd", "pwd", "credential", "auth", "signature", "sign", "sig", "apikey", "api_key", "access_token", "refresh_token",
}

func isSensitiveQueryParam(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "" {
		return false
	}
	for _, keyword := range sensitiveQueryParamKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// MaskSensitiveInfo masks sensitive information in a string.
// For URLs, only query parameters with sensitive names (e.g. key, token,
// secret, password) are masked; the host, path and other query parameters
// are preserved as-is for debuggability. Bare domains and IPs are still
// masked.
// Example:
// https://api.test.org/v1/users/123?key=secret -> https://api.test.org/v1/users/123?key=***
// https://api.test.org/v1/chat?model=gpt-4 -> https://api.test.org/v1/chat?model=gpt-4
// https://sub.domain.co.uk/path?id=1&token=abc -> https://sub.domain.co.uk/path?id=1&token=***
// 192.168.1.1 -> ***.***.***.***
// api_key:AIza*** -> api_key:***
func MaskSensitiveInfo(str string) string {
	// Mask sensitive query parameters in URLs
	str = maskURLPattern.ReplaceAllStringFunc(str, func(urlStr string) string {
		u, err := url.Parse(urlStr)
		if err != nil {
			return urlStr
		}

		if u.RawQuery == "" {
			return urlStr
		}

		values, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			// If can't parse query, mask the whole query string
			return u.Scheme + "://" + u.Host + u.Path + "?***"
		}

		maskedParams := make([]string, 0, len(values))
		changed := false
		for key, vals := range values {
			if isSensitiveQueryParam(key) {
				maskedParams = append(maskedParams, key+"=***")
				changed = true
				continue
			}
			for _, v := range vals {
				maskedParams = append(maskedParams, key+"="+v)
			}
		}
		if !changed {
			return urlStr
		}

		result := u.Scheme + "://" + u.Host + u.Path
		if len(maskedParams) > 0 {
			result += "?" + strings.Join(maskedParams, "&")
		}
		return result
	})

	// Mask IP addresses
	str = maskIPPattern.ReplaceAllString(str, "***.***.***.***")

	// Mask API keys (e.g., "api_key:AIzaSyAAAaUooTUni8AdaOkSRMda30n_Q4vrV70" -> "api_key:***")
	str = maskApiKeyPattern.ReplaceAllString(str, "${1}api_key:***${3}")

	return str
}
