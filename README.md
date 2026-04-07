# html2block

A Go library to convert HTML strings into a structured, JSON-friendly "Block" format.

## Overview

`html2block` parses HTML and transforms it into a tree of structured objects (blocks). This format is ideal for systems that need a clean, predictable representation of rich text and layout without the complexities of raw HTML.

## Installation

```bash
go get github.com/subiz/html2block
```

## Usage

```go
package main

import (
	"fmt"
	"encoding/json"
	"github.com/subiz/html2block"
)

func main() {
	html := `<div><h1>Hello World</h1><p>This is <b>bold</b> text.</p></div>`
	block := html2block.HTML2Block(html)

	b, _ := json.MarshalIndent(block, "", "  ")
	fmt.Println(string(b))
}
```

## Features

- **Standard Tags:** Supports `div`, `p`, `h1`-`h8`, `a`, `img`, `table`, `ul`, `ol`, `li`, etc.
- **Formatting:** Detects and preserves `bold`, `italic`, `underline`, and `strike` through both tags (`<b>`, `<i>`, etc.) and inline CSS (`font-weight`, etc.).
- **Style Extraction:** Extracts a specific subset of CSS attributes (e.g., `color`, `background`, `margin-top`) into a `style` map.
- **Attribute Mapping:** Preserves HTML attributes in an `attrs` map.
- **Block Collapsing:** Automatically simplifies the structure (e.g., merging nested paragraphs, converting single-child spans to text nodes).
- **Emoji & Mentions:** Specialized handling for Lexical-style emojis and data-driven mentions/dynamic fields.

## Development

To run the tests:

```bash
go test -v .
```
