// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "empty string returns zero",
			timeout: "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "30 seconds",
			timeout: "30s",
			want:    30 * time.Second,
			wantErr: false,
		},
		{
			name:    "5 minutes",
			timeout: "5m",
			want:    5 * time.Minute,
			wantErr: false,
		},
		{
			name:    "1 hour 30 minutes",
			timeout: "1h30m",
			want:    90 * time.Minute,
			wantErr: false,
		},
		{
			name:    "500 milliseconds",
			timeout: "500ms",
			want:    500 * time.Millisecond,
			wantErr: false,
		},
		{
			name:    "invalid string returns error",
			timeout: "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "number without unit returns error",
			timeout: "30",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &Implementation{Timeout: tt.timeout}
			got, err := impl.ParseTimeout()

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}
