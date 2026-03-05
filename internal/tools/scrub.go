package tools

import (
	"regexp"
	"strings"
	"sync"
)

// Credential patterns to scrub from tool output before returning to the LLM.
// Inspired by zeroclaw's credential scrubbing system.
var credentialPatterns = []*regexp.Regexp{
	// OpenAI
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
	// Anthropic
	regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]{20,}`),
	// GitHub personal access tokens
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`ghr_[a-zA-Z0-9]{36}`),
	// AWS
	regexp.MustCompile(`AKIA[A-Z0-9]{16}`),
	// Generic key=value patterns (case-insensitive)
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|bearer|authorization)\s*[:=]\s*["']?\S{8,}["']?`),

	// Connection strings (PostgreSQL, MySQL, MongoDB, Redis, AMQP)
	regexp.MustCompile(`(?i)(postgres|postgresql|mysql|mongodb|redis|amqp)://[^\s"']+`),
	// Generic KEY=/SECRET=/CREDENTIAL= env-var patterns (skip already-redacted [REDACTED] values)
	regexp.MustCompile(`(?i)[A-Z_]*(KEY|SECRET|CREDENTIAL|PRIVATE)[A-Z_]*\s*=\s*[^\[\s]{8,}`),
	// DSN/DATABASE_URL env vars (skip already-redacted values)
	regexp.MustCompile(`(?i)(DSN|DATABASE_URL|REDIS_URL|MONGO_URI)\s*=\s*[^\[\s]{8,}`),
	// VIRTUAL_* env vars (internal runtime config, should not leak)
	regexp.MustCompile(`(?i)VIRTUAL_[A-Z_]+\s*=\s*[^\[\s]{4,}`),
	// Long hex strings (64+ chars) — likely encryption keys, hashes, or secrets
	regexp.MustCompile(`[a-fA-F0-9]{64,}`),
}

const redactedPlaceholder = "[REDACTED]"
const serverIPPlaceholder = "[SERVER_IP]"

// dynamicScrubValues holds runtime-discovered values to scrub (e.g., server IPs).
var (
	dynamicScrubMu     sync.RWMutex
	dynamicScrubValues []string
)

var scrubHintSubstrings = []string{
	"sk-", "sk-ant-",
	"ghp_", "gho_", "ghu_", "ghs_", "ghr_",
	"akia",
	"api_key", "apikey", "token", "secret", "password", "bearer", "authorization",
	"postgres://", "postgresql://", "mysql://", "mongodb://", "redis://", "amqp://",
	"key=", "secret=", "credential", "private",
	"dsn", "database_url", "redis_url", "mongo_uri",
	"virtual_",
}

// AddDynamicScrubValues adds exact string values to the dynamic scrub list.
// Thread-safe. Deduplicates. Empty strings are ignored.
func AddDynamicScrubValues(values ...string) {
	dynamicScrubMu.Lock()
	defer dynamicScrubMu.Unlock()

	existing := make(map[string]bool, len(dynamicScrubValues))
	for _, v := range dynamicScrubValues {
		existing[v] = true
	}
	for _, v := range values {
		if v != "" && !existing[v] {
			dynamicScrubValues = append(dynamicScrubValues, v)
			existing[v] = true
		}
	}
}

// DynamicScrubCount returns the number of dynamic scrub values registered.
func DynamicScrubCount() int {
	dynamicScrubMu.RLock()
	defer dynamicScrubMu.RUnlock()
	return len(dynamicScrubValues)
}

// ResetDynamicScrubValues clears all dynamic scrub values. For testing only.
func ResetDynamicScrubValues() {
	dynamicScrubMu.Lock()
	defer dynamicScrubMu.Unlock()
	dynamicScrubValues = nil
}

// ScrubCredentials replaces known credential patterns and dynamic values in text.
func ScrubCredentials(text string) string {
	if text == "" {
		return text
	}

	if shouldRunPatternScrub(text) {
		for _, pat := range credentialPatterns {
			text = pat.ReplaceAllString(text, redactedPlaceholder)
		}
	}

	// Dynamic values (server IPs, etc.)
	dynamicScrubMu.RLock()
	vals := append([]string(nil), dynamicScrubValues...)
	dynamicScrubMu.RUnlock()

	for _, v := range vals {
		if v != "" && strings.Contains(text, v) {
			text = strings.ReplaceAll(text, v, serverIPPlaceholder)
		}
	}

	return text
}

func shouldRunPatternScrub(text string) bool {
	for _, hint := range scrubHintSubstrings {
		if containsASCIIFold(text, hint) {
			return true
		}
	}
	return hasLongHexRun(text, 64)
}

func containsASCIIFold(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	last := len(s) - len(sub)
	for i := 0; i <= last; i++ {
		matched := true
		for j := 0; j < len(sub); j++ {
			if toLowerASCII(s[i+j]) != toLowerASCII(sub[j]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func toLowerASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func hasLongHexRun(s string, minRun int) bool {
	run := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'f') ||
			(c >= 'A' && c <= 'F') {
			run++
			if run >= minRun {
				return true
			}
			continue
		}
		run = 0
	}
	return false
}
