// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package binding

// ErrorCategory represents the type of an error.
type ErrorCategory string

const (
	ErrorCategoryDeserialization ErrorCategory = "deserialization"
	ErrorCategoryValidation      ErrorCategory = "validation"
)

type (
	// Errors may be generated during deserialization, binding, or validation. This
	// type is mapped to the context so you can inject it into your own handlers and
	// use it in your application if you want all your errors to look the same.
	Errors []Error

	// Error is an error with a category.
	Error struct {
		// Category is the type of the error.
		Category ErrorCategory `json:"category,omitempty"`
		// Err is the underlying error.
		Err error `json:"error,omitempty"`
	}
)
