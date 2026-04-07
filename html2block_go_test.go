package html2block

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/subiz/header"
)

func normalize(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		newMap := make(map[string]interface{})
		for k, v := range val {
			if k == "text" {
				if s, ok := v.(string); ok && s == "" {
					continue
				}
			}
			newMap[k] = normalize(v)
		}
		return newMap
	case []interface{}:
		newSlice := make([]interface{}, len(val))
		for i, v := range val {
			newSlice[i] = normalize(v)
		}
		return newSlice
	default:
		return v
	}
}

func assertJSON(t *testing.T, actual *header.Block, expectedJSON string) {
	t.Helper()

	// Normalize actual by marshaling to JSON and then unmarshaling into a generic interface
	actualBytes, _ := json.Marshal(actual)
	var aMap interface{}
	if err := json.Unmarshal(actualBytes, &aMap); err != nil {
		t.Fatalf("failed to unmarshal actual: %v", err)
	}
	aMap = normalize(aMap)

	// Normalize expected by unmarshaling the provided JSON string
	var eMap interface{}
	if err := json.Unmarshal([]byte(expectedJSON), &eMap); err != nil {
		t.Fatalf("failed to unmarshal expected: %v", err)
	}
	eMap = normalize(eMap)

	// reflect.DeepEqual on generic maps/slices handles order-independence for map keys
	if !reflect.DeepEqual(aMap, eMap) {
		aPretty, _ := json.MarshalIndent(aMap, "", "  ")
		ePretty, _ := json.MarshalIndent(eMap, "", "  ")
		t.Errorf("JSON mismatch.\nActual:\n%s\nExpected:\n%s", string(aPretty), string(ePretty))
	}
}

// testHtmlToBlock1
func TestHTML2Block_TableFile(t *testing.T) {
	html, err := os.ReadFile("test-data/minvoice_table.html")
	if err != nil {
		t.Fatalf("failed to read html file: %v", err)
	}
	expectedJS, err := os.ReadFile("test-data/minvoice_table.json")
	if err != nil {
		t.Fatalf("failed to read json file: %v", err)
	}

	out := HTML2Block(string(html))
	assertJSON(t, out, string(expectedJS))
}

// testHtmlToBlock2
func TestHTML2Block_Basic(t *testing.T) {
	out := HTML2Block("xin chào")
	expected := `{"type":"div","content":[{"type":"text","text":"xin chào"}]}`
	assertJSON(t, out, expected)
}

// testHtmlToBlock3
func TestHTML2Block_Formatting(t *testing.T) {
	html := "<div><h1><i>xin chào</i></h1><a href=\"google.com\">here</a></div>"
	out := HTML2Block(html)
	expected := `{
		"type": "div",
		"content": [
			{"type": "heading", "level": 1, "content": [{"type": "text", "text": "xin chào", "italic": true}]},
			{"type": "link", "href": "google.com", "text": "here"}
		]
	}`
	assertJSON(t, out, expected)
}

// testHtmlToBlock4
func TestHTML2Block_HeadingID(t *testing.T) {
	out := HTML2Block("<h1 id=\"hhh\">xin chào</h1>")
	expected := `{
		"type": "div",
		"content": [{"type": "heading", "level": 1, "content": [{"type": "text", "text": "xin chào"}], "id": "hhh"}]
	}`
	assertJSON(t, out, expected)
}

// testHtmlToBlock5
func TestHTML2Block_DivClass(t *testing.T) {
	out := HTML2Block("<div class=\"muted\">xin chào</div>")
	expected := `{"type": "div", "class": "muted", "content": [{"type": "text", "text": "xin chào"}]}`
	assertJSON(t, out, expected)
}

// testHtmlToBlock6
func TestHTML2Block_SpanIDClass(t *testing.T) {
	out := HTML2Block("<span class=\"muted\" id=\"thanh\">xin chào</span>")
	expected := `{"type": "div", "content": [{"type": "text", "id": "thanh", "class": "muted", "text": "xin chào"}]}`
	assertJSON(t, out, expected)
}

// testHtmlToBlock7
func TestHTML2Block_NestedFormatting(t *testing.T) {
	out := HTML2Block("<span class=\"muted\"><i id=\"thanh\" class=\"red\">xin chào</i></span>")
	expected := `{
		"type": "div",
		"content": [{"type": "text", "id": "thanh", "class": "muted red", "text": "xin chào", "italic": true}]
	}`
	assertJSON(t, out, expected)
}

// testHtmlToBlock8
func TestHTML2Block_Attributes(t *testing.T) {
	out := HTML2Block("<div data-md-component=\"skip\"><a href=\"https://subiz.com\" class=\"md-skip\">Skip</a></div>")
	expected := `{
		"type": "div",
		"attrs": {"data-md-component": "skip"},
		"content": [{"type": "link", "href": "https://subiz.com", "class": "md-skip", "text": "Skip"}]
	}`
	assertJSON(t, out, expected)
}

// testHtmlToBlock9
func TestHTML2Block_EntitiesAndStyle(t *testing.T) {
	html := `<span style="color:#000000;font-weight:700;text-decoration:none;vertical-align:baseline;font-size:12pt;font-family:&quot;Arial&quot;;font-style:normal">L&agrave;m sao &#273;&#7875; ki&#7875;m tra t&igrave;nh tr&#7841;ng &#273;&#417;n h&agrave;ng c&#7911;a m&igrave;nh?</span>`
	out := HTML2Block(html)
	expected := `{
          "content": [
            {
              "bold": true,
              "style": {
                "color": "#000000",
                "font_family": "\"Arial\"",
                "font_size": "12pt",
                "font_style": "normal",
                "font_weight": "700"
              },
              "text": "Làm sao để kiểm tra tình trạng đơn hàng của mình?",
              "type": "text"
            }
          ],
          "type": "div"
        }`
	assertJSON(t, out, expected)
}

func TestHTML2Block_BR(t *testing.T) {
	html := "xin chào<br>thanh"
	out := HTML2Block(html)
	expected := `{
		"type": "div",
		"content": [
			{"type": "text", "text": "xin chào"},
			{"type": "text", "text": "\n"},
			{"type": "text", "text": "thanh"}
		]
	}`
	assertJSON(t, out, expected)
}

func TestHTML2Block_Style(t *testing.T) {
	html := `<div style="color: red; border-radius: 4px;">Hello</div>`
	out := HTML2Block(html)
	expected := `{
		"type": "div",
		"style": {"color": "red", "border_radius": "4px"},
		"content": [{"type": "text", "text": "Hello"}]
	}`
	assertJSON(t, out, expected)
}

func TestHTML2Block_EmptyTextEquivalence(t *testing.T) {
	out := &header.Block{Type: "div", Text: ""}
	// assertJSON should treat {"text": ""} as {} because of normalize
	assertJSON(t, out, `{"type": "div", "text": ""}`)
	assertJSON(t, out, `{"type": "div"}`)
}
