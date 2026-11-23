package llm

import "testing"

func TestValidSanitizedTelegramHTML(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		valid bool
	}{
		{"plain text", "Go team!", true},
		{"escaped text and nested styles", `<B><em>Go &amp; cheer!</em> &lt;3</b>`, true},
		{"styles inside and outside a link", `<i><a href="https://example.com"><tg-spoiler>cheer</tg-spoiler></a></i>`, true},
		{"quoted angles are part of a link", `<a href='https://example.com/?q=<b>&other=1'>cheer</A>`, true},
		{"code and pre as separate entities", `<code>dishes</code><pre>Go &lt;team&gt;</pre>`, true},
		{"pre with a code wrapper", `<PRE><CODE>Go team!</code></pre>`, true},
		{"unclosed bold", `<b>Go team!`, false},
		{"misnested styles", `<b><i>Go team!</b></i>`, false},
		{"closing tag without opener", `Go team!</b>`, false},
		{"nested links through a style", `<a href="https://outer.example"><b><a href='https://inner.example'>cheer</a></b></a>`, false},
		{"style inside code", `<code><b>Go team!</b></code>`, false},
		{"pre inside a style", `<b><pre>Go team!</pre></b>`, false},
		{"code wrapper only covers part of pre", `<pre><code>Go</code> team!</pre>`, false},
		{"raw markup outside allowed tags", `<b>Go <team></b>`, false},
		{"NUL in a link", "<a href=\"https://example.com/\x00\">cheer</a>", false},
		{"formatting without a message", "<b> </b>\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidSanitizedTelegramHTML(tt.input); got != tt.valid {
				t.Errorf("ValidSanitizedTelegramHTML(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}
