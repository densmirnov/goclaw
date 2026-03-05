package telegram

import "testing"

var benchTelegramInput = "# Header\n\n> quote\n\n**bold** and _italic_ and ~~strike~~\n\n| Col A | Col B |\n|------|------|\n| one | two |\n| three | four |\n\nInline: `x := 1`\n\n```go\nfmt.Println(\"hello\")\n```\n\nLink: [example](https://example.com)\n"

func BenchmarkMarkdownToTelegramHTML(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = markdownToTelegramHTML(benchTelegramInput)
	}
}
