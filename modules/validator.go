package modules

import "github.com/go-playground/validator/v10"

// Validator is a globals validator
var Validator *validator.Validate

func init() {
	Validator = validator.New()
}
