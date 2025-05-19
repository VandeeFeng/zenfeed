// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package jsonschema

import (
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestForType(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		inputType reflect.Type
	}
	type thenExpected struct {
		schema    map[string]any
		hasError  bool
		errorText string
	}

	type SimpleStruct struct {
		Name        string `json:"name" desc:"The name field"`
		Age         int    `json:"age"`
		IsActive    bool   `json:"is_active"`
		IgnoreField string `json:"-"`
	}

	type EmbeddedStruct struct {
		ID string `json:"id"`
	}

	type ComplexStruct struct {
		EmbeddedStruct
		Time     time.Time         `json:"time"`
		Duration time.Duration     `json:"duration"`
		Tags     []string          `json:"tags"`
		Metadata map[string]string `json:"metadata"`
	}

	type Node struct {
		Value    string `json:"value"`
		Next     *Node  `json:"next"`
		Children []Node `json:"children"`
	}

	type LinkedList struct {
		Head *Node `json:"head"`
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Generate schema for simple struct",
			When:     "providing a struct with basic types",
			Then:     "should generate correct JSON schema",
			WhenDetail: whenDetail{
				inputType: reflect.TypeOf(SimpleStruct{}),
			},
			ThenExpected: thenExpected{
				schema: map[string]any{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"definitions": map[string]any{
						"SimpleStruct": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{
									"type":        "string",
									"description": "The name field",
								},
								"age": map[string]any{
									"type": "integer",
								},
								"is_active": map[string]any{
									"type": "boolean",
								},
							},
						},
					},
					"$ref": "#/definitions/SimpleStruct",
				},
			},
		},
		{
			Scenario: "Generate schema for complex struct",
			When:     "providing a struct with embedded fields and special types",
			Then:     "should generate correct JSON schema with all fields",
			WhenDetail: whenDetail{
				inputType: reflect.TypeOf(ComplexStruct{}),
			},
			ThenExpected: thenExpected{
				schema: map[string]any{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"definitions": map[string]any{
						"ComplexStruct": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{
									"type": "string",
								},
								"time": map[string]any{
									"type":   "string",
									"format": "date-time",
								},
								"duration": map[string]any{
									"type":    "string",
									"format":  "duration",
									"pattern": "^([0-9]+(s|m|h))+$",
								},
								"tags": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type": "string",
									},
								},
								"metadata": map[string]any{
									"type": "object",
									"additionalProperties": map[string]any{
										"type": "string",
									},
								},
							},
						},
					},
					"$ref": "#/definitions/ComplexStruct",
				},
			},
		},
		{
			Scenario: "Generate schema for struct with circular reference",
			When:     "providing a struct that references itself",
			Then:     "should generate correct JSON schema using $ref",
			WhenDetail: whenDetail{
				inputType: reflect.TypeOf(Node{}),
			},
			ThenExpected: thenExpected{
				schema: map[string]any{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"definitions": map[string]any{
						"Node": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"value": map[string]any{
									"type": "string",
								},
								"next": map[string]any{
									"$ref": "#/definitions/Node",
								},
								"children": map[string]any{
									"type": "array",
									"items": map[string]any{
										"$ref": "#/definitions/Node",
									},
								},
							},
						},
					},
					"$ref": "#/definitions/Node",
				},
			},
		},
		{
			Scenario: "Generate schema for struct with nested circular reference",
			When:     "providing a struct that contains a circular reference",
			Then:     "should generate correct JSON schema using $ref",
			WhenDetail: whenDetail{
				inputType: reflect.TypeOf(LinkedList{}),
			},
			ThenExpected: thenExpected{
				schema: map[string]any{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"definitions": map[string]any{
						"LinkedList": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"head": map[string]any{
									"$ref": "#/definitions/Node",
								},
							},
						},
						"Node": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"value": map[string]any{
									"type": "string",
								},
								"next": map[string]any{
									"$ref": "#/definitions/Node",
								},
								"children": map[string]any{
									"type": "array",
									"items": map[string]any{
										"$ref": "#/definitions/Node",
									},
								},
							},
						},
					},
					"$ref": "#/definitions/LinkedList",
				},
			},
		},
		{
			Scenario: "Generate schema for nil type",
			When:     "providing a nil type",
			Then:     "should return error",
			WhenDetail: whenDetail{
				inputType: nil,
			},
			ThenExpected: thenExpected{
				hasError:  true,
				errorText: "type cannot be nil",
			},
		},
		{
			Scenario: "Generate schema for unsupported map key type",
			When:     "providing a map with non-string key type",
			Then:     "should return error",
			WhenDetail: whenDetail{
				inputType: reflect.TypeOf(map[int]string{}),
			},
			ThenExpected: thenExpected{
				hasError:  true,
				errorText: "unsupported map key type: int (must be string)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			schema, err := ForType(tt.WhenDetail.inputType)

			// Then.
			if tt.ThenExpected.hasError {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.errorText))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(schema).To(Equal(tt.ThenExpected.schema))
			}
		})
	}
}
