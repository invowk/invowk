// SPDX-License-Identifier: MPL-2.0

package invowkfile

type fieldValidatable interface {
	Validate() error
}

func appendFieldError(errs *[]error, err error) {
	if err != nil {
		*errs = append(*errs, err)
	}
}

func appendOptionalValidation[T fieldValidatable](errs *[]error, value T, present bool) {
	if present {
		appendFieldError(errs, value.Validate())
	}
}

func appendEachValidation[T fieldValidatable](errs *[]error, values []T) {
	for i := range values {
		appendFieldError(errs, values[i].Validate())
	}
}
