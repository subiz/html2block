package html2block

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Block represents a json-based description of HTML
type Block struct {
	Type      string            `json:"type"`
	Text      string            `json:"text,omitempty"`
	Bold      bool              `json:"bold,omitempty"`
	Italic    bool              `json:"italic,omitempty"`
	Underline bool              `json:"underline,omitempty"`
	Strike    bool              `json:"strike,omitempty"`
	Code      bool              `json:"code,omitempty"`
	Level     int               `json:"level,omitempty"`
	Href      string            `json:"href,omitempty"`
	Title     string            `json:"title,omitempty"`
	AltText   string            `json:"alt_text,omitempty"`
	Target    string            `json:"target,omitempty"`
	Class     string            `json:"class,omitempty"`
	ID        string            `json:"id,omitempty"`
	Style     map[string]string `json:"style,omitempty"`
	Attrs     map[string]string `json:"attrs,omitempty"`
	Content   []*Block          `json:"content,omitempty"`
	Image     *ImageInfo        `json:"image,omitempty"`
}

type ImageInfo struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type Options struct {
	SkipClass bool `json:"skip_class"`
	SkipAttr  bool `json:"skip_attr"`
}

var tagTypeMaps = map[string]string{
	"DETAILS": "paragraph",
	"SUMMARY": "paragraph",
	"ARTICLE": "paragraph",
	"MAIN":    "paragraph",
	"LABEL":   "paragraph",
	"BODY":    "paragraph",
	"DIV":     "paragraph",
	"IMG":     "image",
	"P":       "paragraph",
	"A":       "link",
	"LI":      "list_item",
	"OL":      "ordered_list",
	"UL":      "bullet_list",
	"HR":      "horizontal_rule",
	"CODE":    "code",
	"SPAN":    "span",
	"TR":      "table_row",
	"TD":      "table_cell",
	"TH":      "table_cell",
	"THEAD":   "thead",
	"TBODY":   "tbody",
	"TABLE":   "table",
}

var styleAttributes = []string{
	"border_radius", "font_family", "color", "background", "text_align",
	"text_transform", "font_style", "font_weight", "width", "max_width",
	"height", "max_height", "padding_left", "padding_right", "padding_top",
	"padding_bottom", "margin_left", "margin_right", "margin_top", "margin_bottom",
	"position", "object_fit", "line_height", "background_position", "left",
	"right", "top", "bottom", "opacity", "rotate", "blur", "grayscale",
	"flex", "flex_direction", "flex_shrink", "align_items", "justify_content",
	"transform", "font_size", "z_index", "border_bottom", "border_left",
	"border_top", "border_right", "border", "box_shadow", "overflow",
	"overflow_x", "overflow_y", "white_space", "user_select", "pointer_events",
}

var collapsedTypeMaps = map[string]bool{
	"TBODY": true,
	"THEAD": true,
}

type Emoji struct {
	Code string
}

var LexicalEmojiList []Emoji

func HTML2Block(htmlStr string) *Block {
	doc, _ := html.Parse(strings.NewReader(htmlStr))
	body := findBody(doc)
	if body == nil {
		body = doc
	}

	options := Options{}
	output := domToBlock(body, options)
	collapseBlock(nil, output)
	cleanBlock(output)
	output = cleanEmptyP(output)

	if output != nil && output.Type == "paragraph" {
		output.Type = "div"
	}
	return output
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && strings.ToUpper(n.Data) == "BODY" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := findBody(c); res != nil {
			return res
		}
	}
	return nil
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, className string) bool {
	cl := getAttr(n, "class")
	fields := strings.Fields(cl)
	for _, f := range fields {
		if f == className {
			return true
		}
	}
	return false
}

func parseStyle(styleStr string) map[string]string {
	res := make(map[string]string)
	parts := strings.Split(styleStr, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			k = strings.ReplaceAll(k, "-", "_")
			res[k] = v
		}
	}
	return res
}

func getFormatedResultOfDom(n *html.Node) (bold, italic, underline, strike bool) {
	if n == nil || n.Type != html.ElementNode {
		return
	}
	tagName := strings.ToUpper(n.Data)
	bold = tagName == "B" || tagName == "STRONG"
	italic = tagName == "I" || tagName == "EM"
	underline = tagName == "U"
	strike = tagName == "DEL"

	rawStyle := getAttr(n, "style")
	if rawStyle != "" {
		styles := parseStyle(rawStyle)
		if fwStr, ok := styles["font_weight"]; ok {
			fw, _ := strconv.Atoi(fwStr)
			if fw > 400 {
				bold = true
			}
		}
	}
	return
}

func getTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(getTextContent(c))
	}
	return sb.String()
}

func shouldIgnore(tagName string) bool {
	switch tagName {
	case "FOOTER", "HEADER", "SCRIPT", "STYLE", "NAV", "ASIDE", "HEAD", "LINK", "SVG", "META", "INPUT", "BUTTON":
		return true
	}
	return false
}

func domToBlock(n *html.Node, options Options) *Block {
	if n == nil {
		return nil
	}
	if n.Type == html.CommentNode {
		return nil
	}
	if n.Type != html.ElementNode && n.Type != html.TextNode {
		return nil
	}

	bold, italic, underline, strike := getFormatedResultOfDom(n)

	if n.Type == html.TextNode {
		text := n.Data
		if strings.TrimSpace(text) == "" {
			return nil
		}
		o := &Block{
			Type: "text",
			Text: text,
		}
		if bold {
			o.Bold = true
		}
		if italic {
			o.Italic = true
		}
		if underline {
			o.Underline = true
		}
		if strike {
			o.Strike = true
		}
		return o
	}

	tagName := strings.ToUpper(n.Data)
	if shouldIgnore(tagName) {
		return nil
	}

	style := make(map[string]string)
	rawStyle := getAttr(n, "style")
	parsedStyle := parseStyle(rawStyle)
	for _, key := range styleAttributes {
		if val, ok := parsedStyle[key]; ok {
			style[key] = val
		}
	}

	formatedTagNames := map[string]bool{"U": true, "I": true, "B": true, "STRONG": true, "EM": true, "DEL": true}
	if formatedTagNames[tagName] {
		o := &Block{
			Type: "text",
			ID:   getAttr(n, "id"),
			Text: getTextContent(n),
		}
		if len(style) > 0 {
			o.Style = style
		}
		if !options.SkipClass {
			o.Class = getAttr(n, "class")
		}
		if bold {
			o.Bold = true
		}
		if italic {
			o.Italic = true
		}
		if underline {
			o.Underline = true
		}
		if strike {
			o.Strike = true
		}
		return o
	}

	if tagName == "BR" {
		o := &Block{
			Type: "text",
			Text: "\n",
			ID:   getAttr(n, "id"),
		}
		if len(style) > 0 {
			o.Style = style
		}
		if !options.SkipClass {
			o.Class = getAttr(n, "class")
		}
		return o
	}

	customObj := &Block{
		ID:      getAttr(n, "id"),
		Title:   getAttr(n, "title"),
		AltText: getAttr(n, "alt"),
		Href:    getAttr(n, "href"),
	}

	if bold {
		customObj.Bold = true
	}
	if strike {
		customObj.Strike = true
	}
	if italic {
		customObj.Italic = true
	}
	if underline {
		customObj.Underline = true
	}

	if !options.SkipClass {
		customObj.Class = getAttr(n, "class")
	}
	if src := getAttr(n, "src"); src != "" {
		customObj.Image = &ImageInfo{URL: src}
	}

	remainAttrs := make(map[string]string)
	for _, a := range n.Attr {
		if a.Key == "class" || a.Key == "style" || a.Key == "href" || a.Key == "alt" || a.Key == "src" || a.Key == "id" {
			continue
		}
		remainAttrs[a.Key] = a.Val
	}
	if !options.SkipAttr && len(remainAttrs) > 0 {
		customObj.Attrs = remainAttrs
	}

	if tagName == "SPAN" && hasClass(n, "lexical-emoji") {
		for _, emoji := range LexicalEmojiList {
			if hasClass(n, emoji.Code) {
				return &Block{
					Type:  "emoji",
					Attrs: map[string]string{"code": emoji.Code},
				}
			}
		}
		// find first element child
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				return &Block{
					Type:  "emoji",
					Attrs: map[string]string{"code": getTextContent(c)},
				}
			}
		}
	}

	if n.FirstChild == nil {
		if tagName == "SPAN" {
			if getAttr(n, "data-dynamic-field") != "" {
				customObj.Type = "dynamic-field"
				customObj.Text = getTextContent(n)
				return customObj
			}
			if getAttr(n, "data-mention") != "" {
				customObj.Type = "mention"
				customObj.Text = getTextContent(n)
				return customObj
			}
		}

		t := tagTypeMaps[tagName]
		if t == "" {
			t = tagName
		}
		customObj.Type = strings.ToLower(t)
		customObj.Text = getTextContent(n)
		return customObj
	}

	if len(tagName) >= 2 && tagName[0] == 'H' && tagName[1] >= '1' && tagName[1] <= '8' {
		level, _ := strconv.Atoi(tagName[1:])
		customObj.Type = "heading"
		customObj.Level = level
		var content []*Block
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			cb := domToBlock(c, options)
			if cb != nil {
				content = append(content, cb)
			}
		}
		customObj.Content = content
		return customObj
	}

	t := tagTypeMaps[tagName]
	if t == "" {
		t = tagName
	}
	customObj.Type = strings.ToLower(t)
	var content []*Block
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		cb := domToBlock(c, options)
		if cb != nil {
			content = append(content, cb)
		}
	}
	customObj.Content = content
	return customObj
}

func mergeClass(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	m := make(map[string]bool)
	for _, f := range strings.Fields(a) {
		m[f] = true
	}
	for _, f := range strings.Fields(b) {
		m[f] = true
	}
	var res []string
	for k := range m {
		res = append(res, k)
	}
	sort.Strings(res)
	return strings.Join(res, " ")
}

func collapseBlock(parent, block *Block) {
	if block == nil {
		return
	}

	if len(block.Content) > 0 {
		children := make([]*Block, len(block.Content))
		copy(children, block.Content)
		for _, child := range children {
			collapseBlock(block, child)
		}
	}

	if block.Type == "span" {
		if len(block.Content) == 0 {
			block.Type = "text"
		} else if len(block.Content) == 1 && block.Content[0].Type == "text" {
			child := block.Content[0]
			block.Type = "text"
			block.Text = child.Text
			block.Content = nil
			block.Italic = block.Italic || child.Italic
			block.Bold = block.Bold || child.Bold
			block.Strike = block.Strike || child.Strike
			block.Underline = block.Underline || child.Underline
			if block.ID == "" {
				block.ID = child.ID
			}
			block.Class = mergeClass(block.Class, child.Class)
			if child.Style != nil {
				if block.Style == nil {
					block.Style = make(map[string]string)
				}
				for k, v := range child.Style {
					block.Style[k] = v
				}
			}
		} else if len(block.Content) > 1 {
			block.Type = "paragraph"
		}
	}

	if block.Type == "link" {
		if len(block.Content) == 1 && block.Content[0].Type == "text" {
			child := block.Content[0]
			block.Content = nil
			if block.ID == "" {
				block.ID = child.ID
			}
			block.Text = child.Text
			block.Italic = block.Italic || child.Italic
			block.Strike = block.Strike || child.Strike
			block.Bold = block.Bold || child.Bold
			block.Underline = block.Underline || child.Underline
			block.Class = mergeClass(block.Class, child.Class)
			if child.Style != nil {
				if block.Style == nil {
					block.Style = make(map[string]string)
				}
				for k, v := range child.Style {
					block.Style[k] = v
				}
			}
		}
	}

	if block.Type == "paragraph" {
		if len(block.Content) == 1 && block.Content[0].Type == "paragraph" {
			child := block.Content[0]
			id := block.ID
			if id == "" {
				id = child.ID
			}
			class := mergeClass(block.Class, child.Class)
			style := block.Style
			if child.Style != nil {
				if style == nil {
					style = make(map[string]string)
				}
				for k, v := range child.Style {
					style[k] = v
				}
			}

			*block = *child
			block.ID = id
			block.Class = class
			block.Style = style
		}
	}

	if collapsedTypeMaps[strings.ToUpper(block.Type)] && parent != nil {
		var newContents []*Block
		for _, child := range parent.Content {
			if child != block {
				newContents = append(newContents, child)
			} else {
				newContents = append(newContents, block.Content...)
			}
		}
		parent.Content = newContents
	}
}

func cleanBlock(block *Block) {
	if block == nil {
		return
	}
	if len(block.Content) == 0 {
		block.Content = nil
	} else {
		for _, child := range block.Content {
			cleanBlock(child)
		}
	}
	if len(block.Style) == 0 {
		block.Style = nil
	}
	if len(block.Attrs) == 0 {
		block.Attrs = nil
	}
}

func cleanEmptyP(block *Block) *Block {
	if block == nil {
		return nil
	}
	if block.Type == "text" && block.Text == "" {
		return nil
	}
	if block.Type != "paragraph" {
		return block
	}

	var newContent []*Block
	for _, child := range block.Content {
		c := cleanEmptyP(child)
		if c != nil {
			newContent = append(newContent, c)
		}
	}
	block.Content = newContent

	if len(block.Content) == 0 && block.Text == "" {
		return nil
	}
	return block
}

func Md5sum(str string) string {
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}
