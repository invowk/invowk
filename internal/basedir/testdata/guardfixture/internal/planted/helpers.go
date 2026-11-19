// SPDX-License-Identifier: MPL-2.0

package planted

import (
	_ "github.com/OpenPeeDeeP/xdg"      // want: home-helper import
	_ "github.com/adrg/xdg"             // want: home-helper import
	_ "github.com/kirsle/configdir"     // want: home-helper import
	_ "github.com/mitchellh/go-homedir" // want: home-helper import
	_ "github.com/muesli/go-app-paths"  // want: home-helper import
	_ "github.com/shibukawa/configdir"  // want: home-helper import
)
