package app

import (
	"reflect"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "trim spaces around origins",
			in:   "https://a.example, https://b.example",
			want: []string{"https://a.example", "https://b.example"},
		},
		{
			name: "empty string",
			in:   "",
			want: []string{},
		},
		{
			name: "whitespace only",
			in:   "   ",
			want: []string{},
		},
		{
			name: "wildcard preserved",
			in:   "*",
			want: []string{"*"},
		},
		{
			name: "drop empty segments",
			in:   "a,,b,",
			want: []string{"a", "b"},
		},
		{
			name: "trim methods",
			in:   " GET , POST ",
			want: []string{"GET", "POST"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := splitCSV(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitCSV(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
