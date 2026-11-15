// SPDX-License-Identifier: MPL-2.0

package planted

// Literals plants every forbidden string literal.
var Literals = []string{
	"HOME",          // want: literal HOME
	"USERPROFILE",   // want: literal USERPROFILE
	"APPDATA",       // want: literal APPDATA
	"LOCALAPPDATA",  // want: literal LOCALAPPDATA
	"XDG_DATA_HOME", // want: literal XDG_*
	`$HOME/.cache`,  // want: literal $HOME
	"${HOME}/.ssh",  // want: literal ${HOME}
	"home",
	"HOMEPAGE",
	"XDG_lower",
}
