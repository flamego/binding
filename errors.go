// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package binding

// todo
type ErrorCategory string

const (
	ErrorCategoryDeserialization ErrorCategory = "deserialization"
	ErrorCategoryValidation      ErrorCategory = "validation"
)

type (
	// todo: Errors may be generated during deserialization, binding,
	// or validation. This type is mapped to the context so you
	// can inject it into your own handlers and use it in your
	// application if you want all your errors to look the same.
	Errors []Error

	Error struct {
		// todo: The classification is like an error code, convenient to
		// use when processing or categorizing an error programmatically.
		// It may also be called the "kind" of error.
		Category ErrorCategory `json:"category,omitempty"`

		// todo: Message should be human-readable and detailed enough to
		// pinpoint and resolve the problem, but it should be brief. For
		// example, a payload of 100 objects in a JSON array might have
		// an error in the 41st object. The message should help the
		// end user find and fix the error with their request.
		Err error `json:"error,omitempty"`
	}
)

func newErrors() Errors {
	return make(Errors, 0)
}

// todo: Add adds an error associated with the fields indicated
// by fieldNames, with the given classification and message.
func (errs *Errors) Add(category ErrorCategory, err error) {
	*errs = append(*errs,
		Error{
			Category: category,
			Err:      err,
		},
	)
}

// todo: Len returns the number of errors.
func (errs *Errors) Len() int {
	return len(*errs)
}
