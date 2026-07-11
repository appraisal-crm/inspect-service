package handler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is a shared validator. RegisterTagNameFunc makes error messages use
// the json field name (e.g. "notes") instead of the Go field name ("Notes").
var validate = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	return v
}()

// firstValidationError turns the first validation failure into a human message.
func firstValidationError(err error) string {
	ve, ok := err.(validator.ValidationErrors)
	if !ok || len(ve) == 0 {
		return err.Error()
	}
	fe := ve[0]
	switch fe.Tag() {
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", fe.Field(), fe.Param())
	case "min":
		return fmt.Sprintf("%s must be at least %s character(s)", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s character(s)", fe.Field(), fe.Param())
	case "required":
		return fmt.Sprintf("%s is required", fe.Field())
	default:
		return fmt.Sprintf("%s failed validation: %s", fe.Field(), fe.Tag())
	}
}
