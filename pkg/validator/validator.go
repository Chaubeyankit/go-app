package validator

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	validate *validator.Validate
)

func instance() *validator.Validate {
	once.Do(func() {
		validate = validator.New()
		// Register custom validators here
		_ = validate.RegisterValidation("strongpassword", strongPassword)
	})
	return validate
}

// Validate validates a struct and returns a map of field → message, or nil.
func Validate(s interface{}) map[string]string {
	err := instance().Struct(s)
	if err == nil {
		return nil
	}

	errs := make(map[string]string)
	for _, e := range err.(validator.ValidationErrors) {
		errs[toSnakeCase(e.Field())] = fieldMessage(e)
	}
	return errs
}

func fieldMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "too short (min " + e.Param() + " characters)"
	case "max":
		return "too long (max " + e.Param() + " characters)"
	case "strongpassword":
		return "password must contain at least one uppercase, lowercase, digit, and special character"
	default:
		return "invalid value"
	}
}

func strongPassword(fl validator.FieldLevel) bool {
	pw := fl.Field().String()
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range pw {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSpecial
}

func toSnakeCase(s string) string {
	// Simple implementation — use a library like strcase for production
	result := make([]byte, 0, len(s)+4)
	for i, c := range s {
		if c >= 'A' && c <= 'Z' && i > 0 {
			result = append(result, '_')
		}
		result = append(result, byte(c|32)) // toLower
	}
	return string(result)
}
