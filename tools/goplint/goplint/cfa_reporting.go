// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

func useBeforeValidateMessage(targetName, typeName string, crossBlock bool) string {
	if crossBlock {
		return fmt.Sprintf("variable %s of type %s used before Validate() across blocks", targetName, typeName)
	}
	return fmt.Sprintf("variable %s of type %s used before Validate() in same block", targetName, typeName)
}

func unvalidatedCastMessage(typeName string) string {
	return fmt.Sprintf("type conversion to %s from non-constant without Validate() check", typeName)
}

func reportFindingIfNotBaselined(
	pass *analysis.Pass,
	bl *BaselineConfig,
	pos token.Pos,
	category, findingID, message string,
) {
	if bl.ContainsFinding(category, findingID, message) {
		return
	}
	reportDiagnostic(pass, pos, category, findingID, message)
}
