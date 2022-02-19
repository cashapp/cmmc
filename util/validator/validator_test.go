package validator_test

import (
	"testing"

	"github.com/cashapp/cmmc/util/validator"
)

const awsAuthMapRolesSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "aws-auth-map-roles",
  "type": "array",
  "items": { "$ref": "#/$defs/role" },
  "$defs": {
	"role": {
	  "type": "object",
	  "required": [ "rolearn", "username", "groups" ],
	  "properties":  {
		"rolearn": {
		  "type": "string",
		  "pattern": "^arn:aws:iam::\\d+:role/.+"
		},
		"username": {
		  "type": "string"
		},
		"groups": {
		  "type": "array",
		  "items": {
			"type": "string"
		  }
		}
	  }
	}
  }
}`

func TestValidatorBasic(t *testing.T) {
	err := validator.Validate(`{"type":"string"}`, "hi")
	if err != nil {
		t.Errorf("unexpected failure %s", err)
	}

	err = validator.Validate(`{"type"nana"}`, "hi")
	if err == nil {
		t.Error("expected failure but succeeded")
	}
}

func TestValidateAWSAuth(t *testing.T) {
	if err := validator.Validate(awsAuthMapRolesSchema, `
- rolearn: banana
  username: banana
  groups:
  - hi
  - you
`); err == nil {
		t.Errorf("expected this to fail")
	}
}
