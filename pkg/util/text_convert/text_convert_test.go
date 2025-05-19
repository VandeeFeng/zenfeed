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

package textconvert

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestMarkdownToHTML(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		markdown []byte
	}
	type thenExpected struct {
		html []byte
		err  string
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Convert simple markdown to HTML",
			When:     "converting markdown to HTML",
			Then:     "should return correct HTML",
			WhenDetail: whenDetail{
				markdown: []byte("# Hello World"),
			},
			ThenExpected: thenExpected{
				html: []byte("<h1>Hello World</h1>\n"),
			},
		},
		{
			Scenario: "Convert markdown with formatting to HTML",
			When:     "converting markdown text with formatting to HTML",
			Then:     "should return HTML with proper formatting",
			WhenDetail: whenDetail{
				markdown: []byte("**Bold** and *italic* text"),
			},
			ThenExpected: thenExpected{
				html: []byte("<p><strong>Bold</strong> and <em>italic</em> text</p>\n"),
			},
		},
		{
			Scenario: "Convert markdown with links to HTML",
			When:     "converting markdown text with links to HTML",
			Then:     "should return HTML with proper links",
			WhenDetail: whenDetail{
				markdown: []byte("[Link](https://example.com)"),
			},
			ThenExpected: thenExpected{
				html: []byte("<p><a href=\"https://example.com\">Link</a></p>\n"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(_ *testing.T) {
			// When.
			html, err := MarkdownToHTML(tt.WhenDetail.markdown)

			// Then.
			if tt.ThenExpected.err != "" {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
			} else {
				Expect(err).To(BeNil())
				Expect(html).To(Equal(tt.ThenExpected.html))
			}
		})
	}
}

func TestHTMLToMarkdown(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		html []byte
	}
	type thenExpected struct {
		markdown []byte
		err      string
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Convert simple HTML to markdown",
			When:     "converting HTML text to markdown",
			Then:     "should return correct markdown",
			WhenDetail: whenDetail{
				html: []byte("<h1>Hello World</h1>"),
			},
			ThenExpected: thenExpected{
				markdown: []byte("# Hello World"),
			},
		},
		{
			Scenario: "Convert HTML with formatting to markdown",
			When:     "converting HTML text with formatting to markdown",
			Then:     "should return markdown with proper formatting",
			WhenDetail: whenDetail{
				html: []byte("<p><strong>Bold</strong> and <em>italic</em> text</p>"),
			},
			ThenExpected: thenExpected{
				markdown: []byte("**Bold** and _italic_ text"),
			},
		},
		{
			Scenario: "Convert HTML with links to markdown",
			When:     "converting HTML text with links to markdown",
			Then:     "should return markdown with proper links",
			WhenDetail: whenDetail{
				html: []byte("<p><a href=\"https://example.com\">Link</a></p>"),
			},
			ThenExpected: thenExpected{
				markdown: []byte("[Link](https://example.com)"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(_ *testing.T) {
			// When.
			markdown, err := HTMLToMarkdown(tt.WhenDetail.html)

			// Then.
			if tt.ThenExpected.err != "" {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
			} else {
				Expect(err).To(BeNil())
				Expect(markdown).To(Equal(tt.ThenExpected.markdown))
			}
		})
	}
}
