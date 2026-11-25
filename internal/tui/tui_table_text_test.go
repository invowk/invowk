// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"strings"
	"testing"
)

func TestWriteTableText(t *testing.T) {
	t.Parallel()

	thirty := make([][]string, 30)
	var wantThirty strings.Builder
	wantThirty.WriteString("n\n")
	for i := range thirty {
		thirty[i] = []string{fmt.Sprintf("row%02d", i+1)}
		fmt.Fprintf(&wantThirty, "row%02d\n", i+1)
	}

	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		want    string
	}{
		{
			name:    "aligned rows",
			headers: []string{"name", "age", "city"},
			rows:    [][]string{{"Alice", "30", "NYC"}, {"Bob", "25", "LA"}},
			want:    "name   age  city\nAlice  30   NYC\nBob    25   LA\n",
		},
		{
			name:    "short rows and overridden titles",
			headers: []string{"a", "b", "c"},
			rows:    [][]string{{"x", "y", "z"}, {"1"}},
			want:    "a  b  c\nx  y  z\n1\n",
		},
		{
			name:    "every row is printed",
			headers: []string{"n"},
			rows:    thirty,
			want:    wantThirty.String(),
		},
		{
			name:    "wide characters stay aligned",
			headers: []string{"word", "n"},
			rows:    [][]string{{"你好", "1"}, {"hi", "2"}},
			want:    "word  n\n你好  1\nhi    2\n",
		},
		{
			name:    "empty cell keeps its column",
			headers: []string{"a", "b", "c"},
			rows:    [][]string{{"x", "", "z"}},
			want:    "a  b  c\nx     z\n",
		},
		{
			name:    "row wider than the headers",
			headers: []string{"a"},
			rows:    [][]string{{"x", "y"}},
			want:    "a\nx  y\n",
		},
		{
			name:    "control characters become spaces",
			headers: []string{"name", "notes"},
			rows:    [][]string{{"Alice", "line1\nline2"}, {"Bob", "a\tb\x1b[31mc\r"}},
			want:    "name   notes\nAlice  line1 line2\nBob    a b [31mc\n",
		},
		{
			name:    "headers only",
			headers: []string{"name", "age"},
			want:    "name  age\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var b strings.Builder
			if err := WriteTableText(&b, tt.headers, tt.rows); err != nil {
				t.Fatalf("WriteTableText() error = %v", err)
			}
			got := b.String()
			if got != tt.want {
				t.Fatalf("WriteTableText() =\n%q\nwant\n%q", got, tt.want)
			}
			if strings.Contains(got, "\x1b") {
				t.Fatalf("WriteTableText() output holds a control sequence: %q", got)
			}
		})
	}
}
