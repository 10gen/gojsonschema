// Copyright 2017 johandorland ( https://github.com/johandorland )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gojsonschema

import (
	"testing"
	"fmt"
	"time"
	"reflect"
	"os"
	"path/filepath"
	"net/http"
	"io/ioutil"
	"strings"
	"encoding/json"

	"gopkg.in/mgo.v2/bson"
)

type jsonSchemaTest struct {
	Description string `json:"description"`
	// Some tests do not pass yet, so some tests are manually edited to include
	// an extra attribute whether that specific test should be disabled and skipped
	Disabled bool                 `json:"disabled"`
	Schema   interface{}          `json:"schema"`
	Tests    []jsonSchemaTestCase `json:"tests"`
}
type jsonSchemaTestCase struct {
	Description    string      `json:"description"`
	Data           interface{} `json:"data"`
	Valid          bool        `json:"valid"`
	PassValidation bool        `json:"passValidation"`
	ValidateTest   bool 	   `json:"validateTest"`
	Expression     interface{} `json:"expression"`
	FieldPath      []string    `json:"fieldPath"`
}

func TestSuite(t *testing.T) {

	wd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	wd = filepath.Join(wd, "testdata")

	go func() {
		err := http.ListenAndServe(":1234", http.FileServer(http.Dir(filepath.Join(wd, "remotes"))))
		if err != nil {
			panic(err.Error())
		}
	}()

	testDirectories := []string{"draft4", "draft6", "draft7"}

	var files []string
	for _, testDirectory := range testDirectories {
		testFiles, err := ioutil.ReadDir(filepath.Join(wd, testDirectory))

		if err != nil {
			panic(err.Error())
		}

		for _, fileInfo := range testFiles {
			if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), ".json") {
				files = append(files, filepath.Join(wd, testDirectory, fileInfo.Name()))
			}
		}
	}

	for _, filepath := range files {

		file, err := os.Open(filepath)
		if err != nil {
			t.Errorf("Error (%s)\n", err.Error())
		}
		fmt.Println(file.Name())

		var tests []jsonSchemaTest
		d := json.NewDecoder(file)
		d.UseNumber()
		err = d.Decode(&tests)

		if err != nil {
			t.Errorf("Error (%s)\n", err.Error())
		}

		for _, test := range tests {

			if test.Disabled {
				continue
			}

			testSchemaLoader := NewRawLoader(test.Schema)
			testSchema, err := NewSchema(testSchemaLoader, NewNoopEvaluator())

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}

			for _, testCase := range test.Tests {
				testDataLoader := NewRawLoader(testCase.Data)
				result, err := testSchema.Validate(testDataLoader)

				if err != nil {
					t.Errorf("Error (%s)\n", err.Error())
				}

				if result.Valid() != testCase.Valid {
					schemaString, _ := marshalToJsonString(test.Schema)
					testCaseString, _ := marshalToJsonString(testCase.Data)

					t.Errorf("Test failed : %s\n"+
						"%s.\n"+
						"%s.\n"+
						"expects: %t, given %t\n"+
						"Schema: %s\n"+
						"Data: %s\n",
						file.Name(),
						test.Description,
						testCase.Description,
						testCase.Valid,
						result.Valid(),
						*schemaString,
						*testCaseString)
				}
			}
		}
	}
}

func TestBSONTypes(t *testing.T) {

	for _, test := range testCases() {

		testSchemaLoader := NewRawLoader(test.Schema)


		for _, testCase := range test.Tests {
			testDataLoader := NewGoLoader(testCase.Data)

			var testSchema *Schema
			var err error
			if testCase.ValidateTest {
				testSchema, err = NewSchema(testSchemaLoader, &MockValidateEvaluator{
					t: t,
					expectedExpression: testCase.Expression,
					expectedFieldPath: testCase.FieldPath,
					valid: testCase.PassValidation,
				})
			} else {
				testSchema, err = NewSchema(testSchemaLoader, NewNoopEvaluator())
			}
			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}

			result, err := testSchema.Validate(testDataLoader)

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}

			if result.Valid() != testCase.Valid {
				schemaString, _ := marshalToJsonString(test.Schema)
				testCaseString, _ := marshalToJsonString(testCase.Data)

				t.Errorf("Test failed : %s\n"+
					"%s.\n"+
					"expects: %t, given %t\n"+
					"Schema: %s\n"+
					"Data: %s\n",
					test.Description,
					testCase.Description,
					testCase.Valid,
					result.Valid(),
					*schemaString,
					*testCaseString)
			}
		}
	}
}

type MockValidateEvaluator struct {
	t *testing.T
	expectedExpression interface{}
	expectedFieldPath []string
	valid      bool
}

func (evaluator *MockValidateEvaluator) Evaluate(expression interface{}, fieldPath []string) error {
	if !reflect.DeepEqual(expression, evaluator.expectedExpression) {
		evaluator.t.Errorf("Test failed : \nexpected: %v\n actual: %v\n", evaluator.expectedExpression, expression)
	}
	if !reflect.DeepEqual(fieldPath, evaluator.expectedFieldPath) {
		evaluator.t.Errorf("Test failed : \nexpected: %v\n actual: %v\n", evaluator.expectedFieldPath, fieldPath)
	}
	if evaluator.valid {
		return nil
	}
	return fmt.Errorf("validation error")
}

func bsonTypeTestCase(inputType, matchType string, shouldMatch bool) jsonSchemaTestCase {
	data := getTestData(inputType)
	tc := jsonSchemaTestCase{
		Data: data,
		Description: fmt.Sprintf("a %s is a %s", inputType, matchType),
		Valid: shouldMatch,
	}
	if !shouldMatch{
		tc.Description = fmt.Sprintf("a %s is not a %s", inputType, matchType)
	}
	return tc
}

func bsonTestCase(description string, data interface{}, shouldMatch bool) jsonSchemaTestCase {
	return jsonSchemaTestCase{
		Data: data,
		Description: description,
		Valid: shouldMatch,
	}
}

func validateTestCase(description string, data interface{}, shouldMatch bool, validate bool, expectedExpression interface{}, expectedFieldPath []string) jsonSchemaTestCase {
	return jsonSchemaTestCase{
		Data: data,
		Description: description,
		Valid: shouldMatch,
		Expression: expectedExpression,
		FieldPath: expectedFieldPath,
		ValidateTest: true,
		PassValidation: validate,
	}
}

func getTestData(inputType string) interface{} {
	switch(inputType) {
	case TYPE_OBJECT_ID:
		return bson.NewObjectId()
	case TYPE_INT32, TYPE_INT64:
		return 1
	case TYPE_DOUBLE:
		return 1.1
	case TYPE_STRING:
		return "foo"
	case TYPE_OBJECT:
		return map[string]interface{}{}
	case TYPE_ARRAY:
		return []interface{}{1, 2, 3}
	case TYPE_BOOL, TYPE_BOOLEAN:
		return true
	case TYPE_NULL:
		return nil
	case TYPE_REGEX:
		return bson.RegEx{}
	case TYPE_DATE:
		return time.Now()
	case TYPE_DECIMAL128:
		decimal, err := bson.ParseDecimal128("1.5")
		if err != nil {
			panic(err)
		}
		return decimal
	case "bson.D":
		return bson.D{}
	case TYPE_TIMESTAMP:
		return bson.MongoTimestamp(123)
	default:
		panic(fmt.Sprintf("%s is not a supported test type", inputType))
	}
}

func testCases() []jsonSchemaTest {
	validateExpression := map[string]interface{}{
		"%function": map[string]interface{}{
			"name":      "func0",
			"arguments": []string{"%%value"},
		},
	}
	allOfMap := map[string]interface{}{"foo": bson.RegEx{}, "bar": 2}
	allOfBson := bson.D{{"foo", bson.RegEx{}}, {"bar", 2}}

	return []jsonSchemaTest{
		{
			Description: "objectId type matches objectId",
			Schema: map[string]interface{}{"bsonType": "objectId"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_OBJECT_ID, true),
				bsonTypeTestCase(TYPE_INT32, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_OBJECT_ID, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_OBJECT_ID, false),
			},
		},
		{
			Description: "double type matches double",
			Schema: map[string]interface{}{"bsonType": "double"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_DOUBLE, true),
				bsonTypeTestCase(TYPE_STRING, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_DOUBLE, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_DOUBLE, false),
			},
		},
		{
			Description: "string type matches string",
			Schema: map[string]interface{}{"bsonType": "string"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_STRING, true),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_STRING, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_STRING, false),
			},
		},
		{
			Description: "array type matches array",
			Schema: map[string]interface{}{"bsonType": "array"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_ARRAY, true),
				bsonTypeTestCase(TYPE_BOOL, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_ARRAY, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_ARRAY, false),
				bsonTypeTestCase("bson.D", TYPE_ARRAY, false),
			},
		},
		{
			Description: "object type matches object",
			Schema: map[string]interface{}{"bsonType": "object"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_OBJECT, true),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_OBJECT, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_OBJECT, false),
				bsonTypeTestCase("bson.D", TYPE_OBJECT, true),
			},
		},
		{
			Description: "bool type matches bool",
			Schema: map[string]interface{}{"bsonType": "bool"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_BOOL, true),
				bsonTypeTestCase(TYPE_BOOLEAN, TYPE_BOOL, true),
				bsonTypeTestCase(TYPE_NULL, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_BOOL, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_BOOL, false),
				bsonTypeTestCase("bson.D", TYPE_BOOL, false),
			},
		},
		{
			Description: "date type matches date",
			Schema: map[string]interface{}{"bsonType": "date"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_DATE, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_DATE, true),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_DATE, false),
				bsonTypeTestCase("bson.D", TYPE_DATE, false),
			},
		},
		{
			Description: "null type matches null",
			Schema: map[string]interface{}{"bsonType": "null"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_NULL, true),
				bsonTypeTestCase(TYPE_REGEX, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_NULL, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_NULL, false),
				bsonTypeTestCase("bson.D", TYPE_NULL, false),
			},
		},
		{
			Description: "regex type matches regex",
			Schema: map[string]interface{}{"bsonType": "regex"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_REGEX, true),
				bsonTypeTestCase(TYPE_DATE, TYPE_REGEX, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_REGEX, false),
				bsonTypeTestCase("bson.D", TYPE_REGEX, false),
			},
		},
		{
			Description: "int type matches int",
			Schema: map[string]interface{}{"bsonType": "int"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_INT32, true),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_INT32, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_INT32, false),
				bsonTypeTestCase("bson.D", TYPE_INT32, false),
				bsonTypeTestCase(TYPE_TIMESTAMP, TYPE_INT32, false),
			},
		},
		{
			Description: "timestamp type matches timestamp",
			Schema: map[string]interface{}{"bsonType": "timestamp"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_TIMESTAMP, true),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_TIMESTAMP, false),
				bsonTypeTestCase("bson.D", TYPE_TIMESTAMP, false),
				bsonTypeTestCase(TYPE_TIMESTAMP, TYPE_TIMESTAMP, true),
			},
		},
		{
			Description: "long type matches long",
			Schema: map[string]interface{}{"bsonType": "long"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_INT64, true),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_INT64, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_INT64, false),
				bsonTypeTestCase("bson.D", TYPE_INT64, false),
				bsonTypeTestCase(TYPE_TIMESTAMP, TYPE_INT64, true),
			},
		},
		{
			Description: "decimal type matches decimal",
			Schema: map[string]interface{}{"bsonType": "decimal"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_STRING, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_DECIMAL128, true),
				bsonTypeTestCase("bson.D", TYPE_DECIMAL128, false),
				bsonTypeTestCase(TYPE_TIMESTAMP, TYPE_DECIMAL128, false),
			},
		},
		{
			Description: "number type matches number",
			Schema: map[string]interface{}{"bsonType": "number"},
			Tests: []jsonSchemaTestCase{
				bsonTypeTestCase(TYPE_OBJECT_ID, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_INT32, TYPE_NUMBER, true),
				bsonTypeTestCase(TYPE_DOUBLE, TYPE_NUMBER, true),
				bsonTypeTestCase(TYPE_STRING, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_OBJECT, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_ARRAY, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_BOOL, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_NULL, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_REGEX, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_DATE, TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_DECIMAL128, TYPE_NUMBER, true),
				bsonTypeTestCase("bson.D", TYPE_NUMBER, false),
				bsonTypeTestCase(TYPE_TIMESTAMP, TYPE_NUMBER, false),
			},
		},
		{
			Description: "allOf with bson types",
			Schema: map[string]interface{}{"allOf": []interface{}{
				map[string]interface{}{
					"properties": map[string]interface{}{
						"bar": map[string]interface{}{
							"bsonType": TYPE_INT32,
						},
					},
					"required": []interface{}{"bar"},
				},
				map[string]interface{}{
					"properties": map[string]interface{}{
						"foo": map[string]interface{}{
							"bsonType": TYPE_REGEX,
						},
					},
					"required": []interface{}{"foo"},
				},
			}},
			Tests: []jsonSchemaTestCase{
				bsonTestCase("matching types", map[string]interface{}{"foo": bson.RegEx{}, "bar": 2}, true),
				bsonTestCase("wrong type", map[string]interface{}{"foo": "baz", "bar": 2}, false),
				bsonTestCase("matching types with bson.D", bson.D{{"foo", bson.RegEx{}}, {"bar", 2}}, true),
				bsonTestCase("wrong type with bson.D", bson.D{{"foo", "baz"}, {"bar", 2}}, false),
			},
		},
		{
			Description: "anyOf with bson types",
			Schema: map[string]interface{}{"anyOf": []interface{}{
				map[string]interface{}{
					"bsonType": TYPE_OBJECT_ID,
				},
				map[string]interface{}{
					"bsonType": TYPE_ARRAY,
				},
			}},
			Tests: []jsonSchemaTestCase{
				bsonTestCase("matching bson type", bson.NewObjectId(), true),
				bsonTestCase("matching array type", []interface{}{1, 2, 3}, true),
				bsonTestCase("no matching type", "foo", false),
			},
		},
		{
			Description: "oneOf with bson types",
			Schema: map[string]interface{}{"oneOf": []interface{}{
				map[string]interface{}{
					"bsonType": TYPE_INT32,
				},
				map[string]interface{}{
					"minimum": 2,
				},
			}},
			Tests: []jsonSchemaTestCase{
				bsonTestCase("matching bson type", 1, true),
				bsonTestCase("above minimum", 2.5, true),
				bsonTestCase("matching both", 3, false),
			},
		},
		{
			Description: "additionalItems as schema",
			Schema: map[string]interface{}{
				"items": []interface{}{map[string]interface{}{}},
				"additionalItems": map[string]interface{}{"bsonType": TYPE_BOOL},
			},
			Tests: []jsonSchemaTestCase{
				bsonTestCase("additional items match schema", []interface{}{nil, true, false}, true),
				bsonTestCase("additional items do not match schema", []interface{}{nil, true, "hello"}, false),
			},
		},
		{
			Description: "a schema given for items",
			Schema: map[string]interface{}{
				"items": map[string]interface{}{"bsonType": TYPE_DOUBLE},
			},
			Tests: []jsonSchemaTestCase{
				bsonTestCase("valid items", []interface{}{1.1, 2.1, 3.1}, true),
				bsonTestCase("wrong type of items", []interface{}{1.1, "x"}, false),
			},
		},
		{
			Description: "patternProperties validates properties matching a regex",
			Schema: map[string]interface{}{
				"patternProperties": map[string]interface{}{
					"f.*o": map[string]interface{}{"bsonType": TYPE_INT32},
				},
			},
			Tests: []jsonSchemaTestCase{
				bsonTestCase("a single valid match is valid", map[string]interface{}{"foo": 1}, true),
				bsonTestCase("multiple valid matches is valid", map[string]interface{}{"foo": 1, "foooooo": 2}, true),
				bsonTestCase("a single invalid match is invalid", map[string]interface{}{"foo": "bar", "fooooo": 2}, false),
				bsonTestCase("a single valid match is valid with bson.D", bson.D{{"foo", 1}}, true),
				bsonTestCase("multiple valid matches is valid with bson.D", bson.D{{"foo", 1}, {"foooooo", 2}}, true),
				bsonTestCase("a single invalid match is invalid with bson.D", bson.D{{"foo", "bar"}, {"fooooo", 2}}, false),
			},
		},
		{
			Description: "object properties validation",
			Schema: map[string]interface{}{
				"properties": map[string]interface{}{
					"foo": map[string]interface{}{"bsonType": TYPE_INT32},
					"bar": map[string]interface{}{"bsonType": TYPE_STRING},
				},
			},
			Tests: []jsonSchemaTestCase{
				bsonTestCase("both properties present and valid is valid", map[string]interface{}{"foo": 1, "bar": "baz"}, true),
				bsonTestCase("one property invalid is invalid", map[string]interface{}{"foo": 1, "bar": bson.D{}}, false),
				bsonTestCase("both properties present and valid is valid with bson.D", bson.D{{"foo", 1}, {"bar", "baz"}}, true),
				bsonTestCase("one property invalid is invalid with bson.D", bson.D{{"foo", 1}, {"bar", bson.D{}}}, false),
			},
		},
		{
			Description: "with validate on base level",
			Schema: map[string]interface{}{
				"bsonType": "string",
				"validate": validateExpression,
			},
			Tests: []jsonSchemaTestCase{
				validateTestCase("passes when validate is true", "haley", true, true, validateExpression, []string{}),
				validateTestCase("does not pass when validate is false", "haley", false, false, validateExpression, []string{}),
			},
		},
		{
			Description: "with validate and multiple levels",
			Schema: map[string]interface{}{
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"bsonType": TYPE_STRING,
					},
					"info": map[string]interface{}{
						"bsonType": TYPE_OBJECT,
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"bsonType": TYPE_OBJECT_ID,
							},
							"school": map[string]interface{}{
								"bsonType": TYPE_STRING,
								"validate": validateExpression,
							},
						},
					},
				},
			},
			Tests: []jsonSchemaTestCase{
				validateTestCase(
					"passes when validate is true",
					map[string]interface{}{
						"name": "haley",
						"info": map[string]interface{}{"id": bson.NewObjectId(), "school": "UT Austin"},
					},
					true,
					true,
					validateExpression,
					[]string{"info", "school"},
				),
				validateTestCase(
					"does not pass when validate is false",
					map[string]interface{}{
						"name": "haley",
						"info": map[string]interface{}{"id": bson.NewObjectId(), "school": "UT Austin"},
					},
					false,
					false,
					validateExpression,
					[]string{"info", "school"},
				),
				validateTestCase(
					"passes when validate is true with bson.D",
					bson.D{{"name", "haley"}, {"info", bson.D{{"id", bson.NewObjectId()}, {"school", "UT Austin"}}}},
					true,
					true,
					validateExpression,
					[]string{"info", "school"},
				),
				validateTestCase(
					"does not pass when validate is false with bson.D",
					bson.D{{"name", "haley"}, {"info", bson.D{{"id", bson.NewObjectId()}, {"school", "UT Austin"}}}},
					false,
					false,
					validateExpression,
					[]string{"info", "school"},
				),
			},
		},
		{
			Description: "with validate and allOf",
			Schema: map[string]interface{}{"allOf": []interface{}{
				map[string]interface{}{
					"properties": map[string]interface{}{
						"bar": map[string]interface{}{
							"bsonType": TYPE_INT32,
						},
					},
				},
				map[string]interface{}{
					"properties": map[string]interface{}{
						"foo": map[string]interface{}{
							"bsonType": TYPE_REGEX,
							"validate": validateExpression,
						},
					},
				},
			}},
			Tests: []jsonSchemaTestCase{
				validateTestCase("passes when both are true", allOfMap, true, true, validateExpression, []string{"foo"}),
				validateTestCase("does not pass when validate is false", allOfMap, false, false, validateExpression, []string{"foo"}),
				validateTestCase(
					"does not pass when all are not true",
					map[string]interface{}{"foo": bson.RegEx{}, "bar": "hello"},
					false,
					true,
					validateExpression,
					[]string{"foo"},
				),
				validateTestCase("passes when both are true with bson.D", allOfBson, true, true, validateExpression, []string{"foo"}),
				validateTestCase("does not pass when validate is false with bson.D", allOfBson, false, false, validateExpression, []string{"foo"}),
				validateTestCase(
					"does not pass when all are not true with bson.D",
					bson.D{{"foo", bson.RegEx{}}, {"bar", "hello"}},
					false,
					true,
					validateExpression,
					[]string{"foo"},
				),
			},
		},
		{
			Description: "with validate and anyOf",
			Schema: map[string]interface{}{"anyOf": []interface{}{
				map[string]interface{}{
					"bsonType": TYPE_OBJECT_ID,
				},
				map[string]interface{}{
					"bsonType": TYPE_ARRAY,
					"validate": validateExpression,
				},
			}},
			Tests: []jsonSchemaTestCase{
				validateTestCase("passes when one is true", bson.NewObjectId(), true, true, validateExpression, []string{}),
				validateTestCase("passes when one is true but validate on another is false", bson.NewObjectId(), true, false, validateExpression, []string{}),
				validateTestCase("does not pass when validate is false", []interface{}{}, false, false, validateExpression, []string{}),
			},
		},
		{
			Description: "oneOf with bson types",
			Schema: map[string]interface{}{"oneOf": []interface{}{
				map[string]interface{}{
					"bsonType": TYPE_INT32,
				},
				map[string]interface{}{
					"minimum": 2,
					"validate": validateExpression,
				},
			}},
			Tests: []jsonSchemaTestCase{
				validateTestCase("matching bson type", 1, true, true, validateExpression, []string{}),
				validateTestCase("above minimum", 2.5, true, true, validateExpression, []string{}),
				validateTestCase("above minimum but fail validate", 2.5, false, false, validateExpression, []string{}),
				validateTestCase("matching both", 3, false, true, validateExpression, []string{}),
			},
		},
	}
}
