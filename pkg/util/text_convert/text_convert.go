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
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/vandeefeng/zenfeed/pkg/util/buffer"
)

var (
	md2html goldmark.Markdown
	html2md *md.Converter
)

func init() {
	md2html = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	html2md = md.NewConverter("", true, nil)
}

func MarkdownToHTML(md []byte) ([]byte, error) {
	buf := buffer.Get()
	defer buffer.Put(buf)

	if err := md2html.Convert(md, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func HTMLToMarkdown(html []byte) ([]byte, error) {
	res, err := html2md.ConvertBytes(html)
	if err != nil {
		return nil, err
	}

	return res, nil
}
