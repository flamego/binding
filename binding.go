// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package binding

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/go-playground/validator/v10"

	"github.com/flamego/flamego"
)

// JSON returns a middleware handler that injects a new instance of the model
// with populated fields and binding.Errors for possible deserialization,
// binding, or validation errors into the request context. The model instance
// fields are populated by deserializing the JSON payload from the request body.
func JSON(model interface{}) flamego.Handler {
	ensureNotPointer(model)
	validate := validator.New()
	return flamego.ContextInvoker(func(c flamego.Context) {
		errs := NewErrors()
		obj := reflect.New(reflect.TypeOf(model))
		r := c.Request().Request
		if r.Body != nil {
			defer func() { _ = r.Body.Close() }()
			err := json.NewDecoder(r.Body).Decode(obj.Interface())
			if err != nil && err != io.EOF {
				errs.Add(ErrorCategoryDeserialization, err)
			}
		}
		validateAndMap(c, validate, obj, errs)
	})
}

// ensureNotPointer panics if the given value is a pointer.
func ensureNotPointer(model interface{}) {
	if reflect.TypeOf(model).Kind() == reflect.Ptr {
		panic("binding: pointer can not be accepted as binding model")
	}
}

// validateAndMap performs validation and then maps both the model instance and
// possible errors to the request context.
func validateAndMap(c flamego.Context, validate *validator.Validate, obj reflect.Value, errs Errors) {
	err := validate.StructCtx(c.Request().Context(), obj.Interface())
	if err != nil {
		errs.Add(ErrorCategoryValidation, err)
	}
	c.Map(errs, obj.Interface())
}
