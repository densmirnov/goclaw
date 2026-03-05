package tools

import (
	"strings"
	"testing"
)

var benchNoSecret = strings.Repeat("URL: https://example.com\\nStatus: 200\\nContent: hello world\\n", 50)
var benchWithSecret = strings.Repeat("token=sk-abcdefghijklmnopqrstuvwxyz123456\\n", 40)

func BenchmarkScrubCredentials_NoSecret(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ScrubCredentials(benchNoSecret)
	}
}

func BenchmarkScrubCredentials_WithSecret(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ScrubCredentials(benchWithSecret)
	}
}
