package main

import "testing"

func TestLangFromArgs(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--version"}, ""},
		{[]string{"--lang", "zh"}, "zh"},
		{[]string{"--lang=zh"}, "zh"},
		{[]string{"--lang=en"}, "en"},
		{[]string{"-d", "example.com", "--lang", "zh"}, "zh"},
		{[]string{"--lang"}, ""}, // --lang without value
	}
	for _, tt := range tests {
		if got := langFromArgs(tt.args); got != tt.want {
			t.Errorf("langFromArgs(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}
