package models

// JSONValue is a generic type to represent any JSON value.
// This can be a string, number, boolean, null, object, or array.
type JSONValue interface{}

// JSONObject represents a JSON object, which is a map of strings to JSONValues.
type JSONObject map[string]JSONValue

// JSONArray represents a JSON array, which is a slice of JSONValues.
type JSONArray []JSONValue

// IntermediateRepresentation is a structure to hold the parsed JSON data
// in a way that's easy for the analyzer to work with.
// This is a starting point and might evolve.
type IntermediateRepresentation struct {
	Root       JSONValue
	RootIsArray bool // True if the root of the JSON is an array vs an object
	// We might add more fields here, like metadata or detected types,
	// as the analyzer and generator components are built.
}