package request

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

var validate = validator.New()

func ValidateStruct(payload any) error {
	return validate.Struct(payload)
}

func WriteValidationError(c *gin.Context, err error) {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		fieldErrors := make([]FieldError, 0, len(validationErrors))
		for _, fe := range validationErrors {
			fieldErrors = append(fieldErrors, FieldError{
				Field:   fe.Field(),
				Rule:    fe.Tag(),
				Message: validationMessage(fe.Field(), fe.Tag()),
			})
		}
		c.AbortWithStatusJSON(422, gin.H{
			"status_code": 422,
			"message":     "validation error",
			"data":        fieldErrors,
		})
		return
	}
	c.AbortWithStatusJSON(400, gin.H{
		"status_code": 400,
		"message":     err.Error(),
		"data":        nil,
	})
}

func validationMessage(field, rule string) string {
	switch rule {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email"
	case "min":
		return field + " is too short"
	case "max":
		return field + " is too long"
	case "oneof":
		return field + " has an invalid value"
	default:
		return field + " is invalid"
	}
}
