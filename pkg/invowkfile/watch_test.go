// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"testing"
	"time"
)

func TestParseDebounce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		debounce string
		want     time.Duration
		wantErr  bool
	}{
		{
			name:     "empty string returns zero",
			debounce: "",
			want:     0,
			wantErr:  false,
		},
		{
			name:     "500 milliseconds",
			debounce: "500ms",
			want:     500 * time.Millisecond,
			wantErr:  false,
		},
		{
			name:     "1 second",
			debounce: "1s",
			want:     1 * time.Second,
			wantErr:  false,
		},
		{
			name:     "invalid string returns error",
			debounce: "invalid",
			want:     0,
			wantErr:  true,
		},
		{
			name:     "zero duration returns error",
			debounce: "0s",
			want:     0,
			wantErr:  true,
		},
		{
			name:     "negative duration returns error",
			debounce: "-1s",
			want:     0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := &WatchConfig{Debounce: tt.debounce}
			got, err := w.ParseDebounce()

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseDebounce() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseDebounce() = %v, want %v", got, tt.want)
			}
		})
	}
}
