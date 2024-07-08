// Code generated by github.com/apiplustech/genqlient, DO NOT EDIT.

package test

import (
	"github.com/apiplustech/genqlient/graphql"
)

// DefaultInputsResponse is returned by DefaultInputs on success.
type DefaultInputsResponse struct {
	Default bool `json:"default"`
}

// GetDefault returns DefaultInputsResponse.Default, and is useful for accessing the field via an interface.
func (v *DefaultInputsResponse) GetDefault() bool { return v.Default }

type InputWithDefaults struct {
	Field         string `json:"field,omitempty"`
	NullableField string `json:"nullableField,omitempty"`
}

// GetField returns InputWithDefaults.Field, and is useful for accessing the field via an interface.
func (v *InputWithDefaults) GetField() string { return v.Field }

// GetNullableField returns InputWithDefaults.NullableField, and is useful for accessing the field via an interface.
func (v *InputWithDefaults) GetNullableField() string { return v.NullableField }

// __DefaultInputsInput is used internally by genqlient
type __DefaultInputsInput struct {
	Input InputWithDefaults `json:"input"`
}

// GetInput returns __DefaultInputsInput.Input, and is useful for accessing the field via an interface.
func (v *__DefaultInputsInput) GetInput() InputWithDefaults { return v.Input }

// The query or mutation executed by DefaultInputs.
const DefaultInputs_Operation = `
query DefaultInputs ($input: InputWithDefaults!) {
	default(input: $input)
}
`

func DefaultInputs(
	client_ graphql.Client,
	input InputWithDefaults,
) (*DefaultInputsResponse, error) {
	req_ := &graphql.Request{
		OpName: "DefaultInputs",
		Query:  DefaultInputs_Operation,
		Variables: &__DefaultInputsInput{
			Input: input,
		},
	}
	var err_ error

	var data_ DefaultInputsResponse
	resp_ := &graphql.Response{Data: &data_}

	err_ = client_.MakeRequest(
		nil,
		req_,
		resp_,
	)

	return &data_, err_
}

