// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package binding

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/url"
	"reflect"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/flamego/flamego"
	"github.com/flamego/validator"
)

// Options contains options for binding.JSON, binding.Form middleware.
type Options struct {
	// ErrorHandler will be invoked automatically when errors occurred. Default is
	// to do nothing, but handlers may still use binding.Errors and do custom errors
	// handling.
	ErrorHandler flamego.Handler
	// Validator sets a custom validator instead of the default validator.
	Validator *validator.Validate
	// MaxMemory specifies the maximum amount of memory to be allowed when parsing a
	// multipart form. Default is 10 MiB.
	MaxMemory int64
}

// errorHandlerInvoker is an inject.FastInvoker implementation of
// `func(flamego.Context, Errors)`.
type errorHandlerInvoker func(flamego.Context, Errors)

func (invoke errorHandlerInvoker) Invoke(args []interface{}) ([]reflect.Value, error) {
	invoke(args[0].(flamego.Context), args[1].(Errors))
	return nil, nil
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
	err := validate.VarCtx(c.Request().Context(), obj.Interface(), "dive")
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

func parseOptions(opts Options) Options {
	switch v := opts.ErrorHandler.(type) {
	case func(flamego.Context, Errors):
		opts.ErrorHandler = errorHandlerInvoker(v)
	}

	if opts.Validator == nil {
		opts.Validator = validator.New()
	}

	if opts.MaxMemory <= 0 {
		opts.MaxMemory = 10 * 1 << 20 // 10 MiB
	}

	return opts
}

// JSON returns a middleware handler that injects a new instance of the model
// with populated fields and binding.Errors for any deserialization, binding, or
// validation errors into the request context. The model instance fields are
// populated by deserializing the JSON payload from the request body.
func JSON(model interface{}, opts ...Options) flamego.Handler {
	ensureNotPointer(model)

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	opt = parseOptions(opt)

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
		validateAndMap(c, opt.Validator, obj, errs)

		errs = c.Value(reflect.TypeOf(errs)).Interface().(Errors)
		if len(errs) > 0 && opt.ErrorHandler != nil {
			_, err := c.Invoke(opt.ErrorHandler)
			if err != nil {
				panic("binding.JSON: " + err.Error())
			}
		}
	})
}

// YAML returns a middleware handler that injects a new instance of the model
// with populated fields and binding.Errors for any deserialization, binding, or
// validation errors into the request context. The model instance fields are
// populated by deserializing the YAML payload from the request body.
func YAML(model interface{}, opts ...Options) flamego.Handler {
	ensureNotPointer(model)
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	opt = parseOptions(opt)

	return flamego.ContextInvoker(func(c flamego.Context) {
		var errs Errors
		r := c.Request().Request
		obj := reflect.New(reflect.TypeOf(model))
		if r.Body != nil {
			defer func() { _ = r.Body.Close() }()
			err := yaml.NewDecoder(r.Body).Decode(obj.Interface())
			if err != nil {
				errs = append(errs,
					Error{
						Category: ErrorCategoryDeserialization,
						Err:      err,
					},
				)
			}
		}
		validateAndMap(c, opt.Validator, obj, errs)
		errs = c.Value(reflect.TypeOf(errs)).Interface().(Errors)
		if len(errs) > 0 && opt.ErrorHandler != nil {
			_, err := c.Invoke(opt.ErrorHandler)
			if err != nil {
				panic("binding.YAML: " + err.Error())
			}
		}
	})
}

// Form returns a middleware handler that injects a new instance of the model
// with populated fields and binding.Errors for any deserialization, binding, or
// validation errors into the request context. The model instance fields are
// populated by deserializing the payload from both form-urlencoded data request
// body and URL query parameters.
func Form(model interface{}, opts ...Options) flamego.Handler {
	ensureNotPointer(model)

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	opt = parseOptions(opt)

	return flamego.ContextInvoker(func(c flamego.Context) {
		var errs Errors
		r := c.Request().Request
		err := r.ParseForm()
		if err != nil {
			errs = append(errs,
				Error{
					Category: ErrorCategoryDeserialization,
					Err:      err,
				},
			)
		}

		obj := reflect.New(reflect.TypeOf(model))
		errs = mapForm(obj, r.Form, nil, errs)
		validateAndMap(c, opt.Validator, obj, errs)

		errs = c.Value(reflect.TypeOf(errs)).Interface().(Errors)
		if len(errs) > 0 && opt.ErrorHandler != nil {
			_, err := c.Invoke(opt.ErrorHandler)
			if err != nil {
				panic("binding.Form: " + err.Error())
			}
		}
	})
}

// mapForm takes values from the form data and maps them into the struct object.
func mapForm(
	obj reflect.Value,
	form url.Values,
	files map[string][]*multipart.FileHeader,
	errs Errors,
) Errors {
	if obj.Kind() == reflect.Ptr {
		obj = obj.Elem()
	}
	typ := obj.Type()

	for i := 0; i < typ.NumField(); i++ {
		typeField := typ.Field(i)
		structField := obj.Field(i)
		if !structField.CanSet() {
			continue
		}

		if typeField.Type.Kind() == reflect.Ptr && typeField.Anonymous {
			structField.Set(reflect.New(typeField.Type.Elem()))
			errs = mapForm(structField.Elem(), form, files, errs)
			if reflect.DeepEqual(structField.Elem().Interface(), reflect.Zero(structField.Elem().Type()).Interface()) {
				structField.Set(reflect.Zero(structField.Type()))
			}
		} else if typeField.Type.Kind() == reflect.Struct {
			errs = mapForm(structField, form, files, errs)
		}

		fieldName := typeField.Tag.Get("form")
		if fieldName == "" {
			fieldName = typeField.Name
		}

		inputValue, exists := form[fieldName]
		if exists {
			numElems := len(inputValue)
			if structField.Kind() == reflect.Slice && numElems > 0 {
				sliceOf := structField.Type().Elem().Kind()
				slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
				for i := 0; i < numElems; i++ {
					err := setWithProperType(sliceOf, inputValue[i], slice.Index(i), fieldName)
					if err != nil {
						errs = append(errs, *err)
					}
				}
				obj.Field(i).Set(slice)
			} else {
				err := setWithProperType(typeField.Type.Kind(), inputValue[0], structField, fieldName)
				if err != nil {
					errs = append(errs, *err)
				}
			}
			continue
		}

		inputFile, exists := files[fieldName]
		if !exists {
			continue
		}
		fhType := reflect.TypeOf((*multipart.FileHeader)(nil))
		numElems := len(inputFile)
		if structField.Kind() == reflect.Slice && numElems > 0 && structField.Type().Elem() == fhType {
			slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
			for i := 0; i < numElems; i++ {
				slice.Index(i).Set(reflect.ValueOf(inputFile[i]))
			}
			structField.Set(slice)
		} else if structField.Type() == fhType {
			structField.Set(reflect.ValueOf(inputFile[0]))
		}
	}
	return errs
}

// setWithProperType sets the value of an indeterminate type to the matching
// value from the request in the same type, so that not all deserialized values
// have to be strings. Supported types are int, uint, bool, float and string.
func setWithProperType(kind reflect.Kind, val string, field reflect.Value, name string) *Error {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val == "" {
			val = "0"
		}
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return &Error{
				Category: ErrorCategoryDeserialization,
				Err:      fmt.Errorf("field %q cannot parse %q as int", name, val),
			}
		}

		field.SetInt(parsed)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if val == "" {
			val = "0"
		}
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return &Error{
				Category: ErrorCategoryDeserialization,
				Err:      fmt.Errorf("field %q cannot parse %q as uint", name, val),
			}
		}

		field.SetUint(parsed)

	case reflect.Bool:
		if val == "on" {
			field.SetBool(true)
			break
		}

		if val == "" {
			val = "false"
		}
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			return &Error{
				Category: ErrorCategoryDeserialization,
				Err:      fmt.Errorf("field %q cannot parse %q as bool", name, val),
			}
		}

		field.SetBool(parsed)

	case reflect.Float32:
		if val == "" {
			val = "0.0"
		}
		parsed, err := strconv.ParseFloat(val, 32)
		if err != nil {
			return &Error{
				Category: ErrorCategoryDeserialization,
				Err:      fmt.Errorf("field %q cannot parse %q as float32", name, val),
			}
		}

		field.SetFloat(parsed)

	case reflect.Float64:
		if val == "" {
			val = "0.0"
		}
		parsed, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return &Error{
				Category: ErrorCategoryDeserialization,
				Err:      fmt.Errorf("field %q cannot parse %q as float64", name, val),
			}
		}

		field.SetFloat(parsed)

	case reflect.String:
		field.SetString(val)
	}
	return nil
}

// MultipartForm returns a middleware handler that injects a new instance of the
// model with populated fields and binding.Errors for any deserialization,
// binding, or validation errors into the request context. It works much like
// binding.Form except it can parse multipart forms and handle file uploads.
func MultipartForm(model interface{}, opts ...Options) flamego.Handler {
	ensureNotPointer(model)

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	opt = parseOptions(opt)

	return flamego.ContextInvoker(func(c flamego.Context) {
		var errs Errors
		r := c.Request().Request

		// Only parse the form if it has not yet been parsed, see
		// https://github.com/martini-contrib/csrf/issues/6
		if r.MultipartForm == nil {
			mr, err := r.MultipartReader()
			if err != nil {
				errs = append(errs,
					Error{
						Category: ErrorCategoryDeserialization,
						Err:      err,
					},
				)
			} else {
				form, err := mr.ReadForm(opt.MaxMemory)
				if err != nil {
					errs = append(errs,
						Error{
							Category: ErrorCategoryDeserialization,
							Err:      err,
						},
					)
				}
				r.MultipartForm = form
			}
		}

		obj := reflect.New(reflect.TypeOf(model))
		if r.MultipartForm != nil {
			errs = mapForm(obj, r.MultipartForm.Value, r.MultipartForm.File, errs)
		}
		validateAndMap(c, opt.Validator, obj, errs)

		errs = c.Value(reflect.TypeOf(errs)).Interface().(Errors)
		if len(errs) > 0 && opt.ErrorHandler != nil {
			_, err := c.Invoke(opt.ErrorHandler)
			if err != nil {
				panic("binding.MultipartForm: " + err.Error())
			}
		}
	})
}
