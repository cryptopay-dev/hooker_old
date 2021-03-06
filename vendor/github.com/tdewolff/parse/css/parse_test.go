package css // import "github.com/tdewolff/parse/css"

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/tdewolff/test"
)

////////////////////////////////////////////////////////////////

func TestParse(t *testing.T) {
	var parseTests = []struct {
		inline   bool
		css      string
		expected string
	}{
		{true, " x : y ; ", "x:y;"},
		{true, "color: red;", "color:red;"},
		{true, "color : red;", "color:red;"},
		{true, "color: red; border: 0;", "color:red;border:0;"},
		{true, "color: red !important;", "color:red!important;"},
		{true, "color: red ! important;", "color:red!important;"},
		{true, "white-space: -moz-pre-wrap;", "white-space:-moz-pre-wrap;"},
		{true, "display: -moz-inline-stack;", "display:-moz-inline-stack;"},
		{true, "x: 10px / 1em;", "x:10px/1em;"},
		{true, "x: 1em/1.5em \"Times New Roman\", Times, serif;", "x:1em/1.5em \"Times New Roman\",Times,serif;"},
		{true, "x: hsla(100,50%, 75%, 0.5);", "x:hsla(100,50%,75%,0.5);"},
		{true, "x: hsl(100,50%, 75%);", "x:hsl(100,50%,75%);"},
		{true, "x: rgba(255, 238 , 221, 0.3);", "x:rgba(255,238,221,0.3);"},
		{true, "x: 50vmax;", "x:50vmax;"},
		{true, "color: linear-gradient(to right, black, white);", "color:linear-gradient(to right,black,white);"},
		{true, "color: calc(100%/2 - 1em);", "color:calc(100%/2 - 1em);"},
		{true, "color: calc(100%/2--1em);", "color:calc(100%/2--1em);"},
		{false, "<!-- @charset; -->", "<!--@charset;-->"},
		{false, "@media print, screen { }", "@media print,screen{}"},
		{false, "@media { @viewport ; }", "@media{@viewport;}"},
		{false, "@keyframes 'diagonal-slide' {  from { left: 0; top: 0; } to { left: 100px; top: 100px; } }", "@keyframes 'diagonal-slide'{from{left:0;top:0;}to{left:100px;top:100px;}}"},
		{false, "@keyframes movingbox{0%{left:90%;}50%{left:10%;}100%{left:90%;}}", "@keyframes movingbox{0%{left:90%;}50%{left:10%;}100%{left:90%;}}"},
		{false, ".foo { color: #fff;}", ".foo{color:#fff;}"},
		{false, ".foo { ; _color: #fff;}", ".foo{_color:#fff;}"},
		{false, "a { color: red; border: 0; }", "a{color:red;border:0;}"},
		{false, "a { color: red; border: 0; } b { padding: 0; }", "a{color:red;border:0;}b{padding:0;}"},
		{false, "/* comment */", "/* comment */"},

		// extraordinary
		{true, "color: red;;", "color:red;"},
		{true, "color:#c0c0c0", "color:#c0c0c0;"},
		{true, "background:URL(x.png);", "background:URL(x.png);"},
		{true, "filter: progid : DXImageTransform.Microsoft.BasicImage(rotation=1);", "filter:progid:DXImageTransform.Microsoft.BasicImage(rotation=1);"},
		{true, "/*a*/\n/*c*/\nkey: value;", "key:value;"},
		{true, "@-moz-charset;", "@-moz-charset;"},
		{true, "--custom-variable:  (0;)  ;", "--custom-variable:  (0;)  ;"},
		{false, "@import;@import;", "@import;@import;"},
		{false, ".a .b#c, .d<.e { x:y; }", ".a .b#c,.d<.e{x:y;}"},
		{false, ".a[b~=c]d { x:y; }", ".a[b~=c]d{x:y;}"},
		// {false, "{x:y;}", "{x:y;}"},
		{false, "a{}", "a{}"},
		{false, "a,.b/*comment*/ {x:y;}", "a,.b{x:y;}"},
		{false, "a,.b/*comment*/.c {x:y;}", "a,.b.c{x:y;}"},
		{false, "a{x:; z:q;}", "a{x:;z:q;}"},
		{false, "@font-face { x:y; }", "@font-face{x:y;}"},
		{false, "a:not([controls]){x:y;}", "a:not([controls]){x:y;}"},
		{false, "@document regexp('https:.*') { p { color: red; } }", "@document regexp('https:.*'){p{color:red;}}"},
		{false, "@media all and ( max-width:400px ) { }", "@media all and (max-width:400px){}"},
		{false, "@media (max-width:400px) { }", "@media(max-width:400px){}"},
		{false, "@media (max-width:400px)", "@media(max-width:400px);"},
		{false, "@font-face { ; font:x; }", "@font-face{font:x;}"},
		{false, "@-moz-font-face { ; font:x; }", "@-moz-font-face{font:x;}"},
		{false, "@unknown abc { {} lala }", "@unknown abc{{}lala}"},
		{false, "a[x={}]{x:y;}", "a[x={}]{x:y;}"},
		{false, "a[x=,]{x:y;}", "a[x=,]{x:y;}"},
		{false, "a[x=+]{x:y;}", "a[x=+]{x:y;}"},
		{false, ".cla .ss > #id { x:y; }", ".cla .ss>#id{x:y;}"},
		{false, ".cla /*a*/ /*b*/ .ss{}", ".cla .ss{}"},
		{false, "a{x:f(a(),b);}", "a{x:f(a(),b);}"},
		{false, "a{x:y!z;}", "a{x:y!z;}"},
		{false, "[class*=\"column\"]+[class*=\"column\"]:last-child{a:b;}", "[class*=\"column\"]+[class*=\"column\"]:last-child{a:b;}"},
		{false, "@media { @viewport }", "@media{@viewport;}"},
		{false, "table { @unknown }", "table{@unknown;}"},

		// early endings
		{true, "~color:red", "~color:red"},
		{false, "selector{", "selector{"},
		{false, "@media{selector{", "@media{selector{"},

		// bad grammar
		{false, ".foo { *color: #fff;}", ".foo{*color:#fff}"},

		// issues
		{false, "@media print {.class{width:5px;}}", "@media print{.class{width:5px;}}"},                  // #6
		{false, ".class{width:calc((50% + 2em)/2 + 14px);}", ".class{width:calc((50% + 2em)/2 + 14px);}"}, // #7
		{false, ".class [c=y]{}", ".class [c=y]{}"},                                                       // tdewolff/minify#16
		{false, "table{font-family:Verdana}", "table{font-family:Verdana;}"},                              // tdewolff/minify#22

		// go-fuzz
		{false, "@-webkit-", "@-webkit-;"},
	}
	for _, tt := range parseTests {
		output := ""
		p := NewParser(bytes.NewBufferString(tt.css), tt.inline)
		for {
			grammar, _, data := p.Next()
			if grammar == ErrorGrammar {
				if err := p.Err(); err != io.EOF {
					for _, val := range p.Values() {
						data = append(data, val.Data...)
					}
				} else {
					test.Error(t, err, io.EOF, "in "+tt.css)
					break
				}
			} else if grammar == AtRuleGrammar || grammar == BeginAtRuleGrammar || grammar == BeginRulesetGrammar || grammar == DeclarationGrammar || grammar == CustomPropertyGrammar {
				if grammar == DeclarationGrammar || grammar == CustomPropertyGrammar {
					data = append(data, ":"...)
				}
				for _, val := range p.Values() {
					data = append(data, val.Data...)
				}
				if grammar == BeginAtRuleGrammar || grammar == BeginRulesetGrammar {
					data = append(data, "{"...)
				} else if grammar == AtRuleGrammar || grammar == DeclarationGrammar || grammar == CustomPropertyGrammar {
					data = append(data, ";"...)
				}
			}
			output += string(data)
		}
		test.String(t, output, tt.expected, "in "+tt.css)
	}

	test.String(t, ErrorGrammar.String(), "Error")
	test.String(t, AtRuleGrammar.String(), "AtRule")
	test.String(t, BeginAtRuleGrammar.String(), "BeginAtRule")
	test.String(t, EndAtRuleGrammar.String(), "EndAtRule")
	test.String(t, BeginRulesetGrammar.String(), "BeginRuleset")
	test.String(t, EndRulesetGrammar.String(), "EndRuleset")
	test.String(t, DeclarationGrammar.String(), "Declaration")
	test.String(t, TokenGrammar.String(), "Token")
	test.String(t, CommentGrammar.String(), "Comment")
	test.String(t, CustomPropertyGrammar.String(), "CustomProperty")
	test.String(t, GrammarType(100).String(), "Invalid(100)")
}

func TestParseError(t *testing.T) {
	var parseErrorTests = []struct {
		inline   bool
		css      string
		expected error
	}{
		{false, "selector", ErrBadQualifiedRule},
		{true, "color 0", ErrBadDeclaration},
		{true, "--color 0", ErrBadDeclaration},
		{true, "--custom-variable:0", io.EOF},
	}
	for _, tt := range parseErrorTests {
		p := NewParser(bytes.NewBufferString(tt.css), tt.inline)
		for {
			grammar, _, _ := p.Next()
			if grammar == ErrorGrammar {
				test.Error(t, p.Err(), tt.expected, "in "+tt.css)
				break
			}
		}
	}
}

func TestReader(t *testing.T) {
	input := "x:a;"
	p := NewParser(test.NewPlainReader(bytes.NewBufferString(input)), true)
	for {
		grammar, _, _ := p.Next()
		if grammar == ErrorGrammar {
			break
		}
	}
}

////////////////////////////////////////////////////////////////

func ExampleNewParser() {
	p := NewParser(bytes.NewBufferString("color: red;"), true) // false because this is the content of an inline style attribute
	out := ""
	for {
		gt, _, data := p.Next()
		if gt == ErrorGrammar {
			break
		} else if gt == AtRuleGrammar || gt == BeginAtRuleGrammar || gt == BeginRulesetGrammar || gt == DeclarationGrammar {
			out += string(data)
			if gt == DeclarationGrammar {
				out += ":"
			}
			for _, val := range p.Values() {
				out += string(val.Data)
			}
			if gt == BeginAtRuleGrammar || gt == BeginRulesetGrammar {
				out += "{"
			} else if gt == AtRuleGrammar || gt == DeclarationGrammar {
				out += ";"
			}
		} else {
			out += string(data)
		}
	}
	fmt.Println(out)
	// Output: color:red;
}
