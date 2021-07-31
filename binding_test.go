// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package binding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/flamego/flamego"
)

func TestJSON(t *testing.T) {
	t.Run("pointer model", func(t *testing.T) {
		assert.PanicsWithValue(t,
			"binding: pointer can not be accepted as binding model",
			func() {
				type form struct {
					Username string
					Password string
				}
				JSON(&form{})
			},
		)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		type form struct {
			Username string
			Password string
		}

		var got Errors
		f := flamego.New()
		f.Post("/", JSON(form{}), func(errs Errors) {
			got = errs
		})

		resp := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{`))
		assert.Nil(t, err)

		f.ServeHTTP(resp, req)

		want := Errors{
			{
				Category: ErrorCategoryDeserialization,
				Err:      errors.New("unexpected EOF"),
			},
		}
		assert.Equal(t, want, got)
	})

	type address struct {
		Street string `json:"street" validate:"required"`
		City   string `json:"city" validate:"required"`
		Planet string `json:"planet" validate:"required"`
		Phone  string `json:"phone" validate:"required"`
	}
	type user struct {
		FirstName string     `json:"first_name" validate:"required"`
		LastName  string     `json:"last_name" validate:"required"`
		Age       uint8      `json:"age" validate:"gte=0,lte=130"`
		Email     string     `json:"email" validate:"required,email"`
		Addresses []*address `json:"addresses" validate:"required,dive,required"`
	}

	tests := []struct {
		name         string
		body         interface{}
		assertErrors func(t *testing.T, errs Errors)
	}{
		{
			name: "good",
			body: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       17,
				Email:     "logan.smith@example.com",
				Addresses: []*address{
					{
						Street: "404 Broadway",
						City:   "Browser",
						Planet: "Internet",
						Phone:  "886",
					},
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 0)
			},
		},
		{
			name: "required",
			body: user{
				LastName: "Smith",
				Age:      17,
				Email:    "logan.smith@example.com",
				Addresses: []*address{
					{
						Street: "404 Broadway",
						City:   "Browser",
						Planet: "Internet",
						Phone:  "886",
					},
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 1)

				got := fmt.Sprintf("%v", errs[0])
				want := "{validation Key: 'user.FirstName' Error:Field validation for 'FirstName' failed on the 'required' tag}"
				assert.Equal(t, want, got)
			},
		},
		{
			name: "gte-lte",
			body: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       140,
				Email:     "logan.smith@example.com",
				Addresses: []*address{
					{
						Street: "404 Broadway",
						City:   "Browser",
						Planet: "Internet",
						Phone:  "886",
					},
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 1)

				got := fmt.Sprintf("%v", errs[0])
				want := "{validation Key: 'user.Age' Error:Field validation for 'Age' failed on the 'lte' tag}"
				assert.Equal(t, want, got)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.body)
			assert.Nil(t, err)

			var gotForm user
			var gotErrs Errors
			f := flamego.New()
			f.Post("/", JSON(user{}), func(form user, errs Errors) {
				gotForm = form
				gotErrs = errs
			})

			resp := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(encoded))
			assert.Nil(t, err)

			f.ServeHTTP(resp, req)

			test.assertErrors(t, gotErrs)
			assert.Equal(t, test.body, gotForm)
		})
	}
}
