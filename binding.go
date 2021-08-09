// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package binding

import (
	"encoding/json"
	"reflect"

	"github.com/go-playground/validator/v10"

	"github.com/flamego/flamego"
)

// Options is a struct for specifying configuration options for the binding middleware.
type Options struct {
	// ErrorHandler will be invoked when errors occurred.
	ErrorHandler func(flamego.Context, Errors)
}

// JSON returns a middleware handler that injects a new instance of the model
// with populated fields and binding.Errors for any deserialization,
// binding, or validation errors into the request context. The model instance
// fields are populated by deserializing the JSON payload from the request body.
func JSON(model interface{}, opts ...Options) flamego.Handler {
	var option Options
	if len(opts) == 1 {
		option = opts[0]
	}

	ensureNotPointer(model)
	validate := validator.New()
	return flamego.ContextInvoker(func(c flamego.Context) {
		var errs Errors
		obj := reflect.New(reflect.TypeOf(model))
		r := c.Request().Request
		if r.Body != nil {
			defer func() { _ = r.Body.Close() }()
			err := json.NewDecoder(r.Body).Decode(obj.Interface())
			if err != nil {
				errs = append(errs,
					Error{
						Category: ErrorCategoryDeserialization,
						Err:      err,
					},
				)
			}
		}
		validateAndMap(c, validate, obj, errs)

		errs = c.Value(reflect.TypeOf(errs)).Interface().(Errors)
		if len(errs) > 0 && option.ErrorHandler != nil {
			_, err := c.Invoke(option.ErrorHandler)
			if err != nil {
				panic("binding: " + err.Error())
			}
		}
	})
}

// ensureNotPointer panics if the given value is a pointer.
func ensureNotPointer(model interface{}) {
	if reflect.TypeOf(model).Kind() == reflect.Ptr {
		panic("binding: pointer can not be accepted as binding model")
	}
}

// validateAndMap performs validation and then maps both the model instance and
// any errors to the request context.
func validateAndMap(c flamego.Context, validate *validator.Validate, obj reflect.Value, errs Errors) {
	err := validate.StructCtx(c.Request().Context(), obj.Interface())
	if err != nil {
		errs = append(errs,
			Error{
				Category: ErrorCategoryValidation,
				Err:      err,
			},
		)
	}
	c.Map(errs, obj.Elem().Interface())
}
