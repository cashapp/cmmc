package validator

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
	"sigs.k8s.io/yaml"
)

func Validate(jsonSchema string, data string) error {
	json, err := yaml.YAMLToJSON([]byte(data))
	if err != nil {
		return errors.Wrap(err, "failed to parse yaml to json")
	}

	schema, err := gojsonschema.NewSchema(gojsonschema.NewStringLoader(jsonSchema))
	if err != nil {
		return errors.Wrap(err, "failed to create schema")
	}

	result, err := schema.Validate(gojsonschema.NewBytesLoader(json))
	if err != nil {
		return errors.Wrap(err, "validation error")
	}

	if result.Valid() {
		return nil
	}

	return InvalidContentErr(result.Errors())
}

type InvalidContentError struct {
	Errs []gojsonschema.ResultError
}

func (e *InvalidContentError) Error() string {
	return fmt.Sprintf("failed validation with errors: %s", e.Errs)
}

func InvalidContentErr(errs []gojsonschema.ResultError) error {
	return &InvalidContentError{Errs: errs}
}
