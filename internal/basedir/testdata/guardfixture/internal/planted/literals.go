// SPDX-License-Identifier: MPL-2.0

package planted

// Literals plants every forbidden string literal.
var Literals = []string{
	"HOME",                     // want: literal HOME
	"USERPROFILE",              // want: literal USERPROFILE
	"APPDATA",                  // want: literal APPDATA
	"LOCALAPPDATA",             // want: literal LOCALAPPDATA
	"LocalAppData",             // want: literal LOCALAPPDATA
	"HOME=/x",                  // want: literal HOME=
	"USERPROFILE=",             // want: literal USERPROFILE=
	"appdata=C:\\x",            // want: literal APPDATA=
	"LOCALAPPDATA=x",           // want: literal LOCALAPPDATA=
	"XDG_DATA_HOME",            // want: literal XDG_*
	"XDG_CONFIG_HOME=/x",       // want: literal XDG_*
	"XDG_",                     // want: literal XDG_*
	`$HOME/.cache`,             // want: literal $HOME
	"${HOME}/.ssh",             // want: literal ${HOME}
	"$XDG_CONFIG_HOME/invowk",  // want: literal $XDG_*
	"${XDG_CACHE_HOME}/invowk", // want: literal ${XDG_*}
	`$USERPROFILE\.ssh`,        // want: literal $USERPROFILE
	"${APPDATA}\\invowk",       // want: literal ${APPDATA}
	"$LOCALAPPDATA",            // want: literal $LOCALAPPDATA
	`%APPDATA%\invowk`,         // want: literal %APPDATA%
	"%userprofile%",            // want: literal %USERPROFILE%
	"%LOCALAPPDATA%",           // want: literal %LOCALAPPDATA%
	"%HOME%",                   // want: literal %HOME%
	"%XDG_DATA_HOME%",          // want: literal %XDG_*%
	"home",
	"HOME_DIR",
	"HOMEDIR=x",
	"HOMEPAGE",
	"XDG_lower",
	"${HOMEDIR}",
	"100%APPDATA",
}
