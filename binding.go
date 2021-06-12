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

// todo: Json is middleware to deserialize a JSON payload from the request
// into the struct that is passed in. The resulting struct is then
// validated, but no error handling is actually performed here.
// An interface pointer can be added as a second argument in order
// to map the struct to a specific interface.
func JSON(model interface{}) flamego.Handler {
	ensureNotPointer(model)
	validate := validator.New()
	return flamego.ContextInvoker(func(c flamego.Context) {
		errs := newErrors()
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

// todo: Performs validation and combines errors from validation
// with errors from deserialization, then maps both the
// resulting struct and the errors to the context.
func validateAndMap(c flamego.Context, validate *validator.Validate, obj reflect.Value, errs Errors) {
	err := validate.StructCtx(c.Request().Context(), obj.Interface())
	if err != nil {
		errs.Add(ErrorCategoryValidation, err)
	}
	c.Map(errs, obj.Interface())
}
