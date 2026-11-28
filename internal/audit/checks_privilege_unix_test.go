// SPDX-License-Identifier: MPL-2.0

//go:build unix

package audit

import (
	"strings"
	"testing"
)

func TestGroupHasOtherMembersFromReaders(t *testing.T) {
	t.Parallel()

	const usersTable = `
root:x:0:0:root:/root:/bin/bash
alice:x:1000:1000:Alice:/home/alice:/bin/bash
bob:x:1001:1001:Bob:/home/bob:/bin/bash
`
	const sharedUsersTable = `
alice:x:1000:1000:Alice:/home/alice:/bin/bash
bob:x:1001:1000:Bob:/home/bob:/bin/bash
`

	tests := []struct {
		name   string
		passwd string
		group  string
		uid    uint32
		gid    uint32
		want   bool
	}{
		{
			name:   "standard UPG empty supplementary list",
			passwd: usersTable,
			group:  "alice:x:1000:\nbob:x:1001:\n",
			uid:    1000,
			gid:    1000,
			want:   false,
		},
		{
			name:   "UPG listing only owner in group",
			passwd: usersTable,
			group:  "alice:x:1000:alice\n",
			uid:    1000,
			gid:    1000,
			want:   false,
		},
		{
			name:   "shared group via supplementary member",
			passwd: usersTable,
			group:  "alice:x:1000:bob\n",
			uid:    1000,
			gid:    1000,
			want:   true,
		},
		{
			name:   "shared group via owner plus another member",
			passwd: usersTable,
			group:  "alice:x:1000:alice, bob\n",
			uid:    1000,
			gid:    1000,
			want:   true,
		},
		{
			name:   "shared group via another user sharing primary GID",
			passwd: sharedUsersTable,
			group:  "alice:x:1000:\n",
			uid:    1000,
			gid:    1000,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := groupHasOtherMembersFromReaders(
				strings.NewReader(tt.passwd),
				strings.NewReader(tt.group),
				tt.uid,
				tt.gid,
			)
			if got != tt.want {
				t.Errorf("groupHasOtherMembersFromReaders() = %v, want %v", got, tt.want)
			}
		})
	}

	if groupHasOtherMembersFromReaders(nil, nil, 1000, 1000) {
		t.Error("groupHasOtherMembersFromReaders(nil, nil) = true, want false")
	}
}
