// Copyright 2021 Flamego. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package binding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
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

	t.Run("custom error handler", func(t *testing.T) {
		type form struct {
			Username string `validate:"required"`
			Password string `validate:"required"`
		}

		normalHandler := func(rw http.ResponseWriter, errs Errors) {
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte(errs[0].Err.Error()))
		}

		fastInvokerHandler := func(c flamego.Context, errs Errors) {
			c.ResponseWriter().WriteHeader(http.StatusBadRequest)
			_, _ = c.ResponseWriter().Write([]byte(fmt.Sprintf("Oops! Error occurred: %v", errs[0].Err)))
		}

		tests := []struct {
			name       string
			payload    []byte
			handler    flamego.Handler
			statusCode int
			want       string
		}{
			{
				name:       "invalid JSON",
				payload:    []byte("{"),
				handler:    fastInvokerHandler,
				statusCode: http.StatusBadRequest,
				want:       "Oops! Error occurred: unexpected EOF",
			},
			{
				name:       "validation error",
				payload:    []byte(`{"Username": "alice"}`),
				handler:    fastInvokerHandler,
				statusCode: http.StatusBadRequest,
				want:       "Oops! Error occurred: Key: 'form.Password' Error:Field validation for 'Password' failed on the 'required' tag",
			},
			{
				name:       "normal handler",
				payload:    []byte(`{`),
				handler:    normalHandler,
				statusCode: http.StatusBadRequest,
				want:       "unexpected EOF",
			},
			{
				name:       "fast invoker handler",
				payload:    []byte(`{"Username": "alice", "Password": "supersecurepassword"}`),
				handler:    fastInvokerHandler,
				statusCode: http.StatusOK,
				want:       "Hello world",
			},
			{
				name:       "nil handler",
				payload:    []byte(`{`),
				handler:    nil,
				statusCode: http.StatusOK,
				want:       "Hello world",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				f := flamego.New()
				opts := Options{
					ErrorHandler: test.handler,
				}
				f.Post("/", JSON(form{}, opts), func(c flamego.Context) {
					_, _ = c.ResponseWriter().Write([]byte("Hello world"))
				})

				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(test.payload))
				assert.Nil(t, err)

				f.ServeHTTP(resp, req)
				assert.Equal(t, test.statusCode, resp.Code)
				assert.Equal(t, test.want, resp.Body.String())
			})
		}
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
		body         user
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

	t.Run("slice", func(t *testing.T) {
		tests := []struct {
			name         string
			body         []user
			assertErrors func(t *testing.T, errs Errors)
		}{
			{
				name: "normal",
				body: []user{
					{
						FirstName: "Logan",
						LastName:  "Smith",
						Age:       100,
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
					{
						FirstName: "John",
						LastName:  "Wu",
						Age:       21,
						Email:     "john.wu@example.com",
						Addresses: []*address{
							{
								Street: "404 Broadway",
								City:   "Browser",
								Planet: "Internet",
								Phone:  "233",
							},
						},
					},
				},
				assertErrors: func(t *testing.T, errs Errors) {
					assert.Nil(t, errs)
				},
			},
			{
				name: "required",
				body: []user{
					{
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
				},
				assertErrors: func(t *testing.T, errs Errors) {
					assert.Len(t, errs, 1)

					got := fmt.Sprintf("%v", errs[0])
					want := "{validation Key: '[0].FirstName' Error:Field validation for 'FirstName' failed on the 'required' tag}"
					assert.Equal(t, want, got)
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				encoded, err := json.Marshal(test.body)
				assert.Nil(t, err)

				var gotForm []user
				var gotErrs Errors
				f := flamego.New()
				f.Post("/", JSON([]user{}), func(form []user, errs Errors) {
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
	})
}

func TestForm(t *testing.T) {
	t.Run("pointer model", func(t *testing.T) {
		assert.PanicsWithValue(t,
			"binding: pointer can not be accepted as binding model",
			func() {
				type form struct {
					Username string
					Password string
				}
				Form(&form{})
			},
		)
	})

	t.Run("custom error handler", func(t *testing.T) {
		type form struct {
			Username string `validate:"required"`
			Password string `validate:"required"`
		}

		normalHandler := func(rw http.ResponseWriter, errs Errors) {
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte(errs[0].Err.Error()))
		}

		fastInvokerHandler := func(c flamego.Context, errs Errors) {
			c.ResponseWriter().WriteHeader(http.StatusBadRequest)
			_, _ = c.ResponseWriter().Write([]byte(fmt.Sprintf("Oops! Error occurred: %v", errs[0].Err)))
		}

		tests := []struct {
			name       string
			payload    string
			handler    flamego.Handler
			statusCode int
			want       string
		}{
			{
				name:       "validation error",
				payload:    `Username=alice`,
				handler:    fastInvokerHandler,
				statusCode: http.StatusBadRequest,
				want:       "Oops! Error occurred: Key: 'form.Password' Error:Field validation for 'Password' failed on the 'required' tag",
			},
			{
				name:       "normal handler",
				payload:    `Username=alice`,
				handler:    normalHandler,
				statusCode: http.StatusBadRequest,
				want:       "Key: 'form.Password' Error:Field validation for 'Password' failed on the 'required' tag",
			},
			{
				name:       "fast invoker handler",
				payload:    `Username=alice&Password=supersecurepassword`,
				handler:    fastInvokerHandler,
				statusCode: http.StatusOK,
				want:       "Hello world",
			},
			{
				name:       "nil handler",
				payload:    `Username=alice`,
				handler:    nil,
				statusCode: http.StatusOK,
				want:       "Hello world",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				f := flamego.New()
				opts := Options{
					ErrorHandler: test.handler,
				}
				f.Post("/", Form(form{}, opts), func(c flamego.Context) {
					_, _ = c.ResponseWriter().Write([]byte("Hello world"))
				})

				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(test.payload))
				assert.Nil(t, err)

				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				f.ServeHTTP(resp, req)
				assert.Equal(t, test.statusCode, resp.Code)
				assert.Equal(t, test.want, resp.Body.String())
			})
		}
	})

	type address struct {
		Street string `form:"street" validate:"required"`
		City   string `form:"city" validate:"required"`
		Planet string `form:"planet" validate:"required"`
		Phone  string `form:"phone" validate:"required"`
	}
	type user struct {
		FirstName string   `form:"first_name" validate:"required"`
		LastName  string   `form:"last_name" validate:"required"`
		Age       uint8    `form:"age" validate:"gte=0,lte=130"`
		Height    int      `form:"height" validate:"gte=0"`
		Male      bool     `form:"male"`
		Email     string   `form:"email" validate:"required,email"`
		Weight    float32  `form:"weight" validate:"gte=0"`
		Balance   float64  `form:"balance"`
		Address   address  `form:"address" validate:"required,dive,required"`
		IPs       []string `form:"ip" validate:"dive,ip"`
	}

	tests := []struct {
		name         string
		body         string
		want         user
		assertErrors func(t *testing.T, errs Errors)
	}{
		{
			name: "good",
			body: `first_name=Logan&last_name=Smith&age=17&height=170&male=true&email=logan.smith@example.com&weight=60.7&balance=-12.4` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886` +
				`&ip=192.168.1.1`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       17,
				Height:    170,
				Male:      true,
				Email:     "logan.smith@example.com",
				Weight:    60.7,
				Balance:   -12.4,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
				IPs: []string{"192.168.1.1"},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 0)
			},
		},
		{
			name: "bad int",
			body: `first_name=Logan&last_name=Smith&age=17&height=bad&male=true&email=logan.smith@example.com&weight=60.7&balance=-12.4` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       17,
				Height:    0,
				Male:      true,
				Email:     "logan.smith@example.com",
				Weight:    60.7,
				Balance:   -12.4,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 1)

				got := fmt.Sprintf("%v", errs[0])
				want := `{deserialization field "height" cannot parse "bad" as int}`
				assert.Equal(t, want, got)
			},
		},
		{
			name: "bad uint",
			body: `first_name=Logan&last_name=Smith&age=bad&height=170&male=true&email=logan.smith@example.com&weight=60.7&balance=-12.4` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       0,
				Height:    170,
				Male:      true,
				Email:     "logan.smith@example.com",
				Weight:    60.7,
				Balance:   -12.4,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 1)

				got := fmt.Sprintf("%v", errs[0])
				want := `{deserialization field "age" cannot parse "bad" as uint}`
				assert.Equal(t, want, got)
			},
		},
		{
			name: "bad bool",
			body: `first_name=Logan&last_name=Smith&age=17&height=170&male=bad&email=logan.smith@example.com&weight=60.7&balance=-12.4` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       17,
				Height:    170,
				Male:      false,
				Email:     "logan.smith@example.com",
				Weight:    60.7,
				Balance:   -12.4,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 1)

				got := fmt.Sprintf("%v", errs[0])
				want := `{deserialization field "male" cannot parse "bad" as bool}`
				assert.Equal(t, want, got)
			},
		},
		{
			name: "bad float32",
			body: `first_name=Logan&last_name=Smith&age=17&height=170&male=true&email=logan.smith@example.com&weight=bad&balance=-12.4` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       17,
				Height:    170,
				Male:      true,
				Email:     "logan.smith@example.com",
				Weight:    0,
				Balance:   -12.4,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 1)

				got := fmt.Sprintf("%v", errs[0])
				want := `{deserialization field "weight" cannot parse "bad" as float32}`
				assert.Equal(t, want, got)
			},
		},
		{
			name: "bad float64",
			body: `first_name=Logan&last_name=Smith&age=17&height=170&male=true&email=logan.smith@example.com&weight=60.7&balance=bad` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       17,
				Height:    170,
				Male:      true,
				Email:     "logan.smith@example.com",
				Weight:    60.7,
				Balance:   0,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 1)

				got := fmt.Sprintf("%v", errs[0])
				want := `{deserialization field "balance" cannot parse "bad" as float64}`
				assert.Equal(t, want, got)
			},
		},
		{
			name: "default values",
			body: `first_name=Logan&last_name=Smith&age=&height=&male=&email=logan.smith@example.com&weight=&balance=` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       0,
				Height:    0,
				Male:      false,
				Email:     "logan.smith@example.com",
				Weight:    0,
				Balance:   0,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 0)
			},
		},
		{
			name: "bool on",
			body: `first_name=Logan&last_name=Smith&age=17&height=170&male=on&email=logan.smith@example.com&weight=60.7&balance=-12.4` +
				`&street=404 Broadway&city=Browser&planet=Internet&phone=886` +
				`&ip=192.168.1.1`,
			want: user{
				FirstName: "Logan",
				LastName:  "Smith",
				Age:       17,
				Height:    170,
				Male:      true,
				Email:     "logan.smith@example.com",
				Weight:    60.7,
				Balance:   -12.4,
				Address: address{
					Street: "404 Broadway",
					City:   "Browser",
					Planet: "Internet",
					Phone:  "886",
				},
				IPs: []string{"192.168.1.1"},
			},
			assertErrors: func(t *testing.T, errs Errors) {
				assert.Len(t, errs, 0)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotForm user
			var gotErrs Errors
			f := flamego.New()
			f.Post("/", Form(user{}), func(form user, errs Errors) {
				gotForm = form
				gotErrs = errs
			})

			resp := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(test.body))
			assert.Nil(t, err)

			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			f.ServeHTTP(resp, req)

			test.assertErrors(t, gotErrs)
			assert.Equal(t, test.want, gotForm)
		})
	}
}

func TestMultipartForm(t *testing.T) {
	t.Run("pointer model", func(t *testing.T) {
		assert.PanicsWithValue(t,
			"binding: pointer can not be accepted as binding model",
			func() {
				type form struct {
					Username string
					Password string
				}
				MultipartForm(&form{})
			},
		)
	})

	t.Run("custom error handler", func(t *testing.T) {
		type form struct {
			Username string `validate:"required"`
			Password string `validate:"required"`
		}

		normalHandler := func(rw http.ResponseWriter, errs Errors) {
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte(errs[0].Err.Error()))
		}

		fastInvokerHandler := func(c flamego.Context, errs Errors) {
			c.ResponseWriter().WriteHeader(http.StatusBadRequest)
			_, _ = c.ResponseWriter().Write([]byte(fmt.Sprintf("Oops! Error occurred: %v", errs[0].Err)))
		}

		tests := []struct {
			name       string
			fields     map[string]string
			handler    flamego.Handler
			statusCode int
			want       string
		}{
			{
				name: "validation error",
				fields: map[string]string{
					"Username": "alice",
				},
				handler:    fastInvokerHandler,
				statusCode: http.StatusBadRequest,
				want:       "Oops! Error occurred: Key: 'form.Password' Error:Field validation for 'Password' failed on the 'required' tag",
			},
			{
				name: "normal handler",
				fields: map[string]string{
					"Username": "alice",
				},
				handler:    normalHandler,
				statusCode: http.StatusBadRequest,
				want:       "Key: 'form.Password' Error:Field validation for 'Password' failed on the 'required' tag",
			},
			{
				name: "fast invoker handler",
				fields: map[string]string{
					"Username": "alice",
					"Password": "supersecurepassword",
				},
				handler:    fastInvokerHandler,
				statusCode: http.StatusOK,
				want:       "Hello world",
			},
			{
				name: "nil handler",
				fields: map[string]string{
					"Username": "alice",
				},
				handler:    nil,
				statusCode: http.StatusOK,
				want:       "Hello world",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				f := flamego.New()
				opts := Options{
					ErrorHandler: test.handler,
				}
				f.Post("/", MultipartForm(form{}, opts), func(c flamego.Context) {
					_, _ = c.ResponseWriter().Write([]byte("Hello world"))
				})

				var body bytes.Buffer
				w := multipart.NewWriter(&body)
				for k, v := range test.fields {
					assert.Nil(t, w.WriteField(k, v))
				}
				assert.Nil(t, w.Close())

				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodPost, "/", &body)
				assert.Nil(t, err)

				req.Header.Set("Content-Type", w.FormDataContentType())
				f.ServeHTTP(resp, req)
				assert.Equal(t, test.statusCode, resp.Code)
				assert.Equal(t, test.want, resp.Body.String())
			})
		}
	})

	type profile struct {
		FirstName  string                  `form:"first_name" validate:"required"`
		LastName   string                  `form:"last_name" validate:"required"`
		Background *multipart.FileHeader   `form:"background"`
		Pictures   []*multipart.FileHeader `form:"picture"`
	}
	var gotForm profile
	var gotErrs Errors
	f := flamego.New()
	f.Post("/", MultipartForm(profile{}), func(form profile, errs Errors) {
		gotForm = form
		gotErrs = errs
	})

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	assert.Nil(t, w.WriteField("first_name", "Logan"))
	assert.Nil(t, w.WriteField("last_name", "Smith"))

	fw, err := w.CreateFormFile("background", "background.jpg")
	assert.Nil(t, err)
	_, err = fw.Write([]byte("pretend this is a JPG"))
	assert.Nil(t, err)

	for _, name := range []string{"picture1.jpg", "picture2.jpg"} {
		fw, err := w.CreateFormFile("picture", name)
		assert.Nil(t, err)
		_, err = fw.Write([]byte("pretend this is a JPG"))
		assert.Nil(t, err)
	}

	assert.Nil(t, w.Close())

	resp := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/", &body)
	assert.Nil(t, err)

	req.Header.Set("Content-Type", w.FormDataContentType())
	f.ServeHTTP(resp, req)

	assert.Len(t, gotErrs, 0)
	assert.Equal(t, "Logan", gotForm.FirstName)
	assert.Equal(t, "Smith", gotForm.LastName)
	assert.NotNil(t, gotForm.Background)
	assert.Len(t, gotForm.Pictures, 2)
}
