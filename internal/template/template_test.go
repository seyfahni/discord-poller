package template

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		endDate string
		want    string
		wantErr bool
	}{
		{
			name:    "question placeholder",
			text:    "Who's in on {end_date}?",
			endDate: "Thursday, 08 October 2026",
			want:    "Who's in on Thursday, 08 October 2026?",
		},
		{
			name:    "answer placeholder",
			text:    "I can join on {end_date}",
			endDate: "08.10.2026",
			want:    "I can join on 08.10.2026",
		},
		{
			name:    "multiple placeholders",
			text:    "{end_date} is the date; see you on {end_date}",
			endDate: "08.10.2026",
			want:    "08.10.2026 is the date; see you on 08.10.2026",
		},
		{
			name:    "literal escaped braces",
			text:    "Use {{end_date}} as a placeholder",
			endDate: "08.10.2026",
			want:    "Use {end_date} as a placeholder",
		},
		{
			name:    "literal escaped brace pairs",
			text:    "Value: {{{end_date}}}",
			endDate: "08.10.2026",
			want:    "Value: {08.10.2026}",
		},
		{
			name:    "unicode text and grapheme sequences",
			text:    "Bist du am {end_date} dabei? 🎉👨‍👩‍👧‍👦",
			endDate: "08.10.2026",
			want:    "Bist du am 08.10.2026 dabei? 🎉👨‍👩‍👧‍👦",
		},
		{
			name:    "empty text",
			text:    "",
			endDate: "08.10.2026",
			wantErr: true,
		},
		{
			name:    "whitespace text",
			text:    " \t\n ",
			endDate: "08.10.2026",
			wantErr: true,
		},
		{
			name:    "placeholder renders empty text",
			text:    "{end_date}",
			endDate: "",
			wantErr: true,
		},
		{
			name:    "unsupported placeholder",
			text:    "Poll for {weekday}",
			endDate: "08.10.2026",
			wantErr: true,
		},
		{
			name:    "unclosed opening brace",
			text:    "Poll for {end_date",
			endDate: "08.10.2026",
			wantErr: true,
		},
		{
			name:    "unescaped closing brace",
			text:    "Poll for end_date}",
			endDate: "08.10.2026",
			wantErr: true,
		},
		{
			name:    "empty placeholder",
			text:    "Poll for {}",
			endDate: "08.10.2026",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Render(test.text, test.endDate)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Render() = %q, error = nil; want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if got != test.want {
				t.Errorf("Render() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderLengthAfterPlaceholderExpansion(t *testing.T) {
	input := "Poll {end_date}"
	date := "Thursday, 08 October 2026"

	got, err := Render(input, date)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got != "Poll Thursday, 08 October 2026" {
		t.Fatalf("Render() = %q, unexpected rendered value", got)
	}
	if len([]rune(got)) <= len([]rune(input)) {
		t.Errorf("rendered text length = %d, want greater than template length %d", len([]rune(got)), len([]rune(input)))
	}
}

func TestRenderPreservesUnicodeExactly(t *testing.T) {
	input := "✅ {end_date} 👩🏽‍💻"
	got, err := Render(input, "08.10.2026")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(got, "✅ 08.10.2026 👩🏽‍💻") {
		t.Errorf("Render() = %q, unicode content was not preserved", got)
	}
}
