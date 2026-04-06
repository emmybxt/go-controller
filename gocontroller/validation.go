package gocontroller

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var ErrValidation = errors.New("validation failed")
var (
	defaultValidatorMu sync.RWMutex
	defaultValidator   Validator
)

// Validator is a pluggable validation engine.
type Validator interface {
	Validate(any) error
}

// ValidatorFunc adapts a function into a Validator.
type ValidatorFunc func(any) error

func (f ValidatorFunc) Validate(v any) error { return f(v) }

type ValidationError struct {
	Field   string
	Rule    string
	Message string
}

type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ErrValidation.Error()
	}
	var parts []string
	for _, e := range ve {
		parts = append(parts, fmt.Sprintf("%s: %s", e.Field, e.Message))
	}
	return fmt.Sprintf("%s: %s", ErrValidation.Error(), strings.Join(parts, ", "))
}

func (ve ValidationErrors) Unwrap() error {
	return ErrValidation
}

// SetDefaultValidator sets the process-wide default validator used by Validate and new routers.
func SetDefaultValidator(v Validator) {
	if v == nil {
		return
	}
	defaultValidatorMu.Lock()
	defaultValidator = v
	defaultValidatorMu.Unlock()
}

// DefaultValidator returns the process-wide default validator.
func DefaultValidator() Validator {
	defaultValidatorMu.RLock()
	v := defaultValidator
	defaultValidatorMu.RUnlock()
	if v != nil {
		return v
	}

	// Lazy init without holding read lock during construction.
	defaultValidatorMu.Lock()
	defer defaultValidatorMu.Unlock()
	if defaultValidator == nil {
		defaultValidator = NewGoPlaygroundValidator()
	}
	return defaultValidator
}

func Validate(dto any) error {
	return DefaultValidator().Validate(dto)
}

// GoPlaygroundValidator uses github.com/go-playground/validator/v10.
type GoPlaygroundValidator struct {
	once sync.Once
	inst *validator.Validate
}

func NewGoPlaygroundValidator() *GoPlaygroundValidator {
	return &GoPlaygroundValidator{}
}

func (g *GoPlaygroundValidator) Validate(dto any) error {
	if dto == nil {
		return nil
	}
	v := reflect.ValueOf(dto)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	if err := g.get().Struct(dto); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			out := make(ValidationErrors, 0, len(verrs))
			for _, fe := range verrs {
				out = append(out, ValidationError{
					Field:   fe.Field(),
					Rule:    fe.Tag(),
					Message: validationMessage(fe.Tag(), fe.Param()),
				})
			}
			return out
		}
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}

	return nil
}

func (g *GoPlaygroundValidator) get() *validator.Validate {
	g.once.Do(func() {
		v := validator.New()
		v.RegisterTagNameFunc(func(field reflect.StructField) string {
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				return field.Name
			}
			return name
		})
		g.inst = v
	})
	return g.inst
}

func validationMessage(tag, param string) string {
	switch tag {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be >= %s", param)
	case "max":
		return fmt.Sprintf("must be <= %s", param)
	case "oneof":
		return fmt.Sprintf("must be one of [%s]", param)
	default:
		if param != "" {
			return fmt.Sprintf("failed %s=%s validation", tag, param)
		}
		return fmt.Sprintf("failed %s validation", tag)
	}
}
