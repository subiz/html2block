package html2block

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/subiz/header"
	"golang.org/x/net/html"
)

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

func HTML2Block(htmlStr string) *header.Block {
	htmlStr = strings.TrimSpace(htmlStr)
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

	if output == nil {
		return &header.Block{}
	}

	if output.Type == "paragraph" {
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

func domToBlock(n *html.Node, options Options) *header.Block {
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
		o := &header.Block{
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
			o.StrikeThrough = true
		}
		return o
	}

	tagName := strings.ToUpper(n.Data)
	if shouldIgnore(tagName) {
		return nil
	}

	style := &header.Style{}
	rawStyle := getAttr(n, "style")
	parsedStyle := parseStyle(rawStyle)
	for _, key := range styleAttributes {
		if val, ok := parsedStyle[key]; ok {
			setStyleField(style, key, val)
		}
	}

	formatedTagNames := map[string]bool{"U": true, "I": true, "B": true, "STRONG": true, "EM": true, "DEL": true}
	if formatedTagNames[tagName] {
		o := &header.Block{
			Type: "text",
			Id:   getAttr(n, "id"),
			Text: getTextContent(n),
		}
		if !isStyleEmpty(style) {
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
			o.StrikeThrough = true
		}
		return o
	}

	if tagName == "BR" {
		o := &header.Block{
			Type: "text",
			Text: "\n",
			Id:   getAttr(n, "id"),
		}
		if !isStyleEmpty(style) {
			o.Style = style
		}
		if !options.SkipClass {
			o.Class = getAttr(n, "class")
		}
		return o
	}

	customObj := &header.Block{
		Id:      getAttr(n, "id"),
		Title:   getAttr(n, "title"),
		AltText: getAttr(n, "alt"),
		Href:    getAttr(n, "href"),
	}

	if !isStyleEmpty(style) {
		customObj.Style = style
	}

	if bold {
		customObj.Bold = true
	}
	if strike {
		customObj.StrikeThrough = true
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
		customObj.Image = &header.File{Url: src}
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
				return &header.Block{
					Type:  "emoji",
					Attrs: map[string]string{"code": emoji.Code},
				}
			}
		}
		// find first element child
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				return &header.Block{
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
		customObj.Level = int64(level)
		var content []*header.Block
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
	var content []*header.Block
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

func collapseBlock(parent, block *header.Block) {
	if block == nil {
		return
	}

	if len(block.Content) > 0 {
		children := make([]*header.Block, len(block.Content))
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
			block.StrikeThrough = block.StrikeThrough || child.StrikeThrough
			block.Underline = block.Underline || child.Underline
			if block.Id == "" {
				block.Id = child.Id
			}
			block.Class = mergeClass(block.Class, child.Class)
			if child.Style != nil {
				if block.Style == nil {
					block.Style = &header.Style{}
				}
				mergeStyle(block.Style, child.Style)
			}
		} else if len(block.Content) > 1 {
			block.Type = "paragraph"
		}
	}

	if block.Type == "link" {
		if len(block.Content) == 1 && block.Content[0].Type == "text" {
			child := block.Content[0]
			block.Content = nil
			if block.Id == "" {
				block.Id = child.Id
			}
			block.Text = child.Text
			block.Italic = block.Italic || child.Italic
			block.StrikeThrough = block.StrikeThrough || child.StrikeThrough
			block.Bold = block.Bold || child.Bold
			block.Underline = block.Underline || child.Underline
			block.Class = mergeClass(block.Class, child.Class)
			if child.Style != nil {
				if block.Style == nil {
					block.Style = &header.Style{}
				}
				mergeStyle(block.Style, child.Style)
			}
		}
	}

	if block.Type == "paragraph" {
		if len(block.Content) == 1 && block.Content[0].Type == "paragraph" {
			child := block.Content[0]
			id := block.Id
			if id == "" {
				id = child.Id
			}
			class := mergeClass(block.Class, child.Class)
			style := block.Style
			if child.Style != nil {
				if style == nil {
					style = &header.Style{}
				}
				mergeStyle(style, child.Style)
			}

			*block = *child
			block.Id = id
			block.Class = class
			block.Style = style
		}
	}

	if collapsedTypeMaps[strings.ToUpper(block.Type)] && parent != nil {
		var newContents []*header.Block
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

func cleanBlock(block *header.Block) {
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
	if isStyleEmpty(block.Style) {
		block.Style = nil
	}
	if len(block.Attrs) == 0 {
		block.Attrs = nil
	}
}

func cleanEmptyP(block *header.Block) *header.Block {
	if block == nil {
		return nil
	}
	if block.Type == "text" && block.Text == "" {
		return nil
	}
	if block.Type != "paragraph" {
		return block
	}

	var newContent []*header.Block
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

func setStyleField(s *header.Style, key, value string) {
	switch key {
	case "border_radius":
		s.BorderRadius = value
	case "font_family":
		s.FontFamily = value
	case "color":
		s.Color = value
	case "background":
		s.Background = value
	case "text_align":
		s.TextAlign = value
	case "text_transform":
		s.TextTransform = value
	case "font_style":
		s.FontStyle = value
	case "font_weight":
		s.FontWeight = value
	case "width":
		s.Width = value
	case "max_width":
		s.MaxWidth = value
	case "height":
		s.Height = value
	case "max_height":
		s.MaxHeight = value
	case "padding_left":
		s.PaddingLeft = value
	case "padding_right":
		s.PaddingRight = value
	case "padding_top":
		s.PaddingTop = value
	case "padding_bottom":
		s.PaddingBottom = value
	case "margin_left":
		s.MarginLeft = value
	case "margin_right":
		s.MarginRight = value
	case "margin_top":
		s.MarginTop = value
	case "margin_bottom":
		s.MarginBottom = value
	case "position":
		s.Position = value
	case "object_fit":
		s.ObjectFit = value
	case "line_height":
		s.LineHeight = value
	case "background_position":
		s.BackgroundPosition = value
	case "left":
		s.Left = value
	case "right":
		s.Right = value
	case "top":
		s.Top = value
	case "bottom":
		s.Bottom = value
	case "opacity":
		s.Opacity = value
	case "rotate":
		s.Rotate = value
	case "blur":
		s.Blur = value
	case "grayscale":
		s.Grayscale = value
	case "flex":
		s.Flex = value
	case "flex_direction":
		s.FlexDirection = value
	case "flex_shrink":
		s.FlexShrink = value
	case "align_items":
		s.AlignItems = value
	case "justify_content":
		s.JustifyContent = value
	case "transform":
		s.Transform = value
	case "font_size":
		s.FontSize = value
	case "z_index":
		s.ZIndex = value
	case "border_bottom":
		s.BorderBottom = value
	case "border_left":
		s.BorderLeft = value
	case "border_top":
		s.BorderTop = value
	case "border_right":
		s.BorderRight = value
	case "border":
		s.Border = value
	case "box_shadow":
		s.BoxShadow = value
	case "overflow":
		s.Overflow = value
	case "overflow_x":
		s.OverflowX = value
	case "overflow_y":
		s.OverflowY = value
	case "white_space":
		s.WhiteSpace = value
	case "user_select":
		s.UserSelect = value
	case "pointer_events":
		s.PointerEvents = value
	}
}

func isStyleEmpty(s *header.Style) bool {
	if s == nil {
		return true
	}
	return s.BorderRadius == "" && s.FontFamily == "" && s.Color == "" && s.Background == "" && s.TextAlign == "" &&
		s.TextTransform == "" && s.FontStyle == "" && s.FontWeight == "" && s.Width == "" && s.MaxWidth == "" &&
		s.Height == "" && s.MaxHeight == "" && s.PaddingLeft == "" && s.PaddingRight == "" && s.PaddingTop == "" &&
		s.PaddingBottom == "" && s.MarginLeft == "" && s.MarginRight == "" && s.MarginTop == "" && s.MarginBottom == "" &&
		s.Position == "" && s.ObjectFit == "" && s.LineHeight == "" && s.BackgroundPosition == "" && s.Left == "" &&
		s.Right == "" && s.Top == "" && s.Bottom == "" && s.Opacity == "" && s.Rotate == "" && s.Blur == "" &&
		s.Grayscale == "" && s.Flex == "" && s.FlexDirection == "" && s.FlexShrink == "" && s.AlignItems == "" &&
		s.JustifyContent == "" && s.Transform == "" && s.FontSize == "" && s.ZIndex == "" && s.BorderBottom == "" &&
		s.BorderLeft == "" && s.BorderTop == "" && s.BorderRight == "" && s.Border == "" && s.BoxShadow == "" &&
		s.Overflow == "" && s.OverflowX == "" && s.OverflowY == "" && s.WhiteSpace == "" && s.UserSelect == "" &&
		s.PointerEvents == "" && s.Hover == nil
}

func mergeStyle(dst, src *header.Style) {
	if src == nil || dst == nil {
		return
	}
	if src.BorderRadius != "" {
		dst.BorderRadius = src.BorderRadius
	}
	if src.FontFamily != "" {
		dst.FontFamily = src.FontFamily
	}
	if src.Color != "" {
		dst.Color = src.Color
	}
	if src.Background != "" {
		dst.Background = src.Background
	}
	if src.TextAlign != "" {
		dst.TextAlign = src.TextAlign
	}
	if src.TextTransform != "" {
		dst.TextTransform = src.TextTransform
	}
	if src.FontStyle != "" {
		dst.FontStyle = src.FontStyle
	}
	if src.FontWeight != "" {
		dst.FontWeight = src.FontWeight
	}
	if src.Width != "" {
		dst.Width = src.Width
	}
	if src.MaxWidth != "" {
		dst.MaxWidth = src.MaxWidth
	}
	if src.Height != "" {
		dst.Height = src.Height
	}
	if src.MaxHeight != "" {
		dst.MaxHeight = src.MaxHeight
	}
	if src.PaddingLeft != "" {
		dst.PaddingLeft = src.PaddingLeft
	}
	if src.PaddingRight != "" {
		dst.PaddingRight = src.PaddingRight
	}
	if src.PaddingTop != "" {
		dst.PaddingTop = src.PaddingTop
	}
	if src.PaddingBottom != "" {
		dst.PaddingBottom = src.PaddingBottom
	}
	if src.MarginLeft != "" {
		dst.MarginLeft = src.MarginLeft
	}
	if src.MarginRight != "" {
		dst.MarginRight = src.MarginRight
	}
	if src.MarginTop != "" {
		dst.MarginTop = src.MarginTop
	}
	if src.MarginBottom != "" {
		dst.MarginBottom = src.MarginBottom
	}
	if src.Position != "" {
		dst.Position = src.Position
	}
	if src.ObjectFit != "" {
		dst.ObjectFit = src.ObjectFit
	}
	if src.LineHeight != "" {
		dst.LineHeight = src.LineHeight
	}
	if src.BackgroundPosition != "" {
		dst.BackgroundPosition = src.BackgroundPosition
	}
	if src.Left != "" {
		dst.Left = src.Left
	}
	if src.Right != "" {
		dst.Right = src.Right
	}
	if src.Top != "" {
		dst.Top = src.Top
	}
	if src.Bottom != "" {
		dst.Bottom = src.Bottom
	}
	if src.Opacity != "" {
		dst.Opacity = src.Opacity
	}
	if src.Rotate != "" {
		dst.Rotate = src.Rotate
	}
	if src.Blur != "" {
		dst.Blur = src.Blur
	}
	if src.Grayscale != "" {
		dst.Grayscale = src.Grayscale
	}
	if src.Flex != "" {
		dst.Flex = src.Flex
	}
	if src.FlexDirection != "" {
		dst.FlexDirection = src.FlexDirection
	}
	if src.FlexShrink != "" {
		dst.FlexShrink = src.FlexShrink
	}
	if src.AlignItems != "" {
		dst.AlignItems = src.AlignItems
	}
	if src.JustifyContent != "" {
		dst.JustifyContent = src.JustifyContent
	}
	if src.Transform != "" {
		dst.Transform = src.Transform
	}
	if src.FontSize != "" {
		dst.FontSize = src.FontSize
	}
	if src.ZIndex != "" {
		dst.ZIndex = src.ZIndex
	}
	if src.BorderBottom != "" {
		dst.BorderBottom = src.BorderBottom
	}
	if src.BorderLeft != "" {
		dst.BorderLeft = src.BorderLeft
	}
	if src.BorderTop != "" {
		dst.BorderTop = src.BorderTop
	}
	if src.BorderRight != "" {
		dst.BorderRight = src.BorderRight
	}
	if src.Border != "" {
		dst.Border = src.Border
	}
	if src.BoxShadow != "" {
		dst.BoxShadow = src.BoxShadow
	}
	if src.Overflow != "" {
		dst.Overflow = src.Overflow
	}
	if src.OverflowX != "" {
		dst.OverflowX = src.OverflowX
	}
	if src.OverflowY != "" {
		dst.OverflowY = src.OverflowY
	}
	if src.WhiteSpace != "" {
		dst.WhiteSpace = src.WhiteSpace
	}
	if src.UserSelect != "" {
		dst.UserSelect = src.UserSelect
	}
	if src.PointerEvents != "" {
		dst.PointerEvents = src.PointerEvents
	}
}
