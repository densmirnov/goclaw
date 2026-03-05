package tools

import "testing"

var benchMarkdownInput = "# Title\n\nThis is **bold** and __also bold__ with [a link](https://example.com).\n\n![img](https://example.com/a.png)\n\nInline code: `const x = 1`\n\n## Subtitle\n\nAnother paragraph with ~~strike~~ and list:\n- one\n- two\n"

func BenchmarkMarkdownToText(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = markdownToText(benchMarkdownInput)
	}
}
