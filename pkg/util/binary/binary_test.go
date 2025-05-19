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

package binary

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestWriteString(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		str string
	}
	type thenExpected struct{}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Write empty string",
			When:     "writing an empty string to a buffer",
			Then:     "should write successfully without error",
			WhenDetail: whenDetail{
				str: "",
			},
			ThenExpected: thenExpected{},
		},
		{
			Scenario: "Write normal string",
			When:     "writing a normal string to a buffer",
			Then:     "should write successfully without error",
			WhenDetail: whenDetail{
				str: "hello world",
			},
			ThenExpected: thenExpected{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			buf := &bytes.Buffer{}
			err := WriteString(buf, tt.WhenDetail.str)

			// Then.
			Expect(err).NotTo(HaveOccurred())

			// Verify the written data by reading it back
			readStr, readErr := ReadString(bytes.NewReader(buf.Bytes()))
			Expect(readErr).NotTo(HaveOccurred())
			Expect(readStr).To(Equal(tt.WhenDetail.str))
		})
	}
}
