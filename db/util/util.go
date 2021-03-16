package util

import (
	"encoding/json"

	"github.com/alecthomas/jsonschema"
	core "github.com/textileio/go-threads/core/db"
	"github.com/tidwall/sjson"
)

func SchemaFromInstance(i interface{}, expandedStruct bool) *jsonschema.Schema {
	reflector := jsonschema.Reflector{ExpandedStruct: expandedStruct}
	return reflector.Reflect(i)
}

func SchemaFromSchemaString(s string) *jsonschema.Schema {
	schemaBytes := []byte(s)
	schema := &jsonschema.Schema{}
	if err := json.Unmarshal(schemaBytes, schema); err != nil {
		panic(err)
	}
	return schema
}

func JSONFromInstance(i interface{}) []byte {
	JSON, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return JSON
}

func InstanceFromJSON(b []byte, i interface{}) {
	if err := json.Unmarshal(b, i); err != nil {
		panic(err)
	}
}

func SetJSONProperty(name string, value interface{}, json []byte) []byte {
	updated, err := sjson.SetBytes(json, name, value)
	if err != nil {
		panic(err)
	}
	return updated
}

func SetJSONID(id core.InstanceID, json []byte) []byte {
	return SetJSONProperty("_id", id.String(), json)
}
