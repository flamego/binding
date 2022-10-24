# binding

[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/flamego/binding/Go?logo=github&style=for-the-badge)](https://github.com/flamego/binding/actions?query=workflow%3AGo)
[![Codecov](https://img.shields.io/codecov/c/gh/flamego/binding?logo=codecov&style=for-the-badge)](https://app.codecov.io/gh/flamego/binding)
[![GoDoc](https://img.shields.io/badge/GoDoc-Reference-blue?style=for-the-badge&logo=go)](https://pkg.go.dev/github.com/flamego/binding?tab=doc)
[![Sourcegraph](https://img.shields.io/badge/view%20on-Sourcegraph-brightgreen.svg?style=for-the-badge&logo=sourcegraph)](https://sourcegraph.com/github.com/flamego/binding)

Package binding is a middleware that provides request data binding and validation for [Flamego](https://github.com/flamego/flamego).

## Installation

The minimum requirement of Go is **1.18**.

	go get github.com/flamego/binding

## Getting started

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/flamego/binding"
	"github.com/flamego/flamego"
)

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

func main() {
	f := flamego.Classic()
	f.Post("/", binding.JSON(user{}), func(c flamego.Context, form user, errs binding.Errors) {
		if len(errs) > 0 {
			c.ResponseWriter().WriteHeader(http.StatusBadRequest)
			_, _ = c.ResponseWriter().Write([]byte(fmt.Sprintf("Oops! Error occurred: %v", errs[0].Err)))
			return
		}

		fmt.Printf("Name: %s %s\n", form.FirstName, form.LastName)
	})
	f.Run()
}
```

## Getting help

- Read [documentation and examples](https://flamego.dev/middleware/binding.html).
- Please [file an issue](https://github.com/flamego/flamego/issues) or [start a discussion](https://github.com/flamego/flamego/discussions) on the [flamego/flamego](https://github.com/flamego/flamego) repository.

## License

This project is under the MIT License. See the [LICENSE](LICENSE) file for the full license text.
