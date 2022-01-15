# aws-auth JSONSchema

These satisfy the kube-system/aws-auth definition for `mapUsers` and `mapRoles`.

## mapRoles

```json
{
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
        "username": { "type": "string" },
        "groups": { "type": "array", "items": { "type": "string" } }
      }
    }
  }
}
```

## mapUsers

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "aws-auth-map-users",
  "type": "array",
  "items": { "$ref": "#/$defs/user" },
  "$defs": {
    "user": {
      "type": "object",
      "required": [ "userarn", "username", "groups" ],
      "properties":  {
        "userarn": {
          "type": "string",
          "pattern": "^arn:aws:iam::\\d+:user/.+"
        },
        "username": { "type": "string" },
        "groups": { "type": "array", "items": { "type": "string" } }
      }
    }
  }
}
```
