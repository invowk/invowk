// SPDX-License-Identifier: MPL-2.0

//go:build unix

package audit

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
)

//goplint:ignore -- inspects raw OS filesystem permissions on Unix paths.
func classifyFileWritableRisk(filePath string, isSharedGroup func(os.FileInfo) bool) (writableRisk, fs.FileMode) {
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return writableRiskNone, 0
	}
	mode := info.Mode().Perm()
	if mode&worldWriteMask != 0 {
		return writableRiskWorld, mode
	}
	if mode&groupWriteMask != 0 && isSharedGroup != nil && isSharedGroup(info) {
		return writableRiskGroup, mode
	}
	return writableRiskNone, mode
}

// isUnixSharedGroup reports whether info's owning group may grant write access
// to users other than the file owner. Standard User-Private Groups (UPG, where
// uid == gid and no other passwd/group user belongs to that group) return false
// so Fedora/Silverblue umask 0002 checkouts do not produce false positives.
func isUnixSharedGroup(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return true
	}
	if stat.Uid != stat.Gid {
		return true
	}
	return groupHasOtherMembers(stat.Uid, stat.Gid)
}

// groupHasOtherMembers inspects /etc/passwd and /etc/group (without os/user)
// to verify whether any user other than ownerUID belongs to groupGID.
// If /etc/passwd or /etc/group cannot be opened, uid == gid is treated as a
// private group (returns false) to avoid noisy false positives in minimal roots.
//
//goplint:ignore -- Unix syscall.Stat_t UID/GID integers for /etc/passwd and /etc/group inspection.
func groupHasOtherMembers(ownerUID, groupGID uint32) bool {
	var passwdReader io.Reader
	if f, err := os.Open("/etc/passwd"); err == nil {
		defer func() { _ = f.Close() }() // read-only system file
		passwdReader = f
	}
	var groupReader io.Reader
	if f, err := os.Open("/etc/group"); err == nil {
		defer func() { _ = f.Close() }() // read-only system file
		groupReader = f
	}
	return groupHasOtherMembersFromReaders(passwdReader, groupReader, ownerUID, groupGID)
}

// groupHasOtherMembersFromReaders parses passwd and group streams to report
// whether any account other than ownerUID belongs to groupGID.
//
//goplint:ignore -- parses /etc/passwd and /etc/group streams with raw UID/GID integers.
func groupHasOtherMembersFromReaders(passwdReader, groupReader io.Reader, ownerUID, groupGID uint32) bool {
	ownerName, otherPrimaryMember := inspectPasswdGroupMembership(passwdReader, ownerUID, groupGID)
	if otherPrimaryMember {
		return true
	}
	return inspectGroupSupplementaryMembers(groupReader, groupGID, ownerName)
}

//goplint:ignore -- reads /etc/passwd lines and raw UID/GID integers from syscall.Stat_t.
func inspectPasswdGroupMembership(r io.Reader, ownerUID, groupGID uint32) (ownerName string, otherPrimaryMember bool) {
	if r == nil {
		return "", false
	}
	uidStr := strconv.FormatUint(uint64(ownerUID), 10)
	gidStr := strconv.FormatUint(uint64(groupGID), 10)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 4 {
			continue
		}
		if fields[2] == uidStr {
			ownerName = fields[0]
		} else if fields[3] == gidStr {
			otherPrimaryMember = true
		}
	}
	return ownerName, otherPrimaryMember
}

//goplint:ignore -- reads /etc/group lines and compares raw GID integer and owner account string.
func inspectGroupSupplementaryMembers(r io.Reader, groupGID uint32, ownerName string) bool {
	if r == nil {
		return false
	}
	gidStr := strconv.FormatUint(uint64(groupGID), 10)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 4 || fields[2] != gidStr {
			continue
		}
		if fields[3] == "" {
			return false
		}
		for member := range strings.SplitSeq(fields[3], ",") {
			member = strings.TrimSpace(member)
			if member == "" {
				continue
			}
			if ownerName == "" || member != ownerName {
				return true
			}
		}
		return false
	}
	return false
}
