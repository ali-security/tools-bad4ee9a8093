// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package static

import (
	"bytes"
	"io/ioutil"
	"regexp"
	"runtime"
	"strconv"
	"testing"
	"unicode"
)

func TestStaticIsUpToDate(t *testing.T) {
	if runtime.GOOS == "android" {
		t.Skip("files not available on android")
	}
	oldBuf, err := ioutil.ReadFile("static.go")
	if err != nil {
		t.Errorf("error while reading static.go: %v\n", err)
	}

	newBuf, err := Generate()
	if err != nil {
		t.Errorf("error while generating static.go: %v\n", err)
	}

	if !bytes.Equal(oldBuf, newBuf) {
		t.Error(`static.go is stale.  Run:
  $ go generate golang.org/x/tools/godoc/static
  $ git diff
to see the differences.`)

	}
}

// TestAppendQuote ensures that AppendQuote produces a valid literal.
func TestAppendQuote(t *testing.T) {
	var in, out bytes.Buffer
	for r := rune(0); r < unicode.MaxRune; r++ {
		in.WriteRune(r)
	}
	appendQuote(&out, in.Bytes())
	in2, err := strconv.Unquote(out.String())
	if err != nil {
		t.Fatalf("AppendQuote produced invalid string literal: %v", err)
	}
	if got, want := in2, in.String(); got != want {
		t.Fatal("AppendQuote modified string") // no point printing got/want: huge
	}
}

// TestJQuerySecurityFixes ensures the jQuery served by godoc carries the fixes
// for CVE-2012-6708, CVE-2015-9251, CVE-2019-11358, CVE-2020-11022,
// CVE-2020-11023 and CVE-2020-7656.
func TestJQuerySecurityFixes(t *testing.T) {
	js, ok := Files["jquery.js"]
	if !ok {
		t.Fatal("jquery.js not found in Files")
	}
	for _, tc := range []struct {
		name    string
		re      string
		present bool
	}{
		// $("#x <img src=x onerror=...>") must be a selector, not HTML.
		{"CVE-2012-6708 unanchored quickExpr", `\[\^#<\]\*\(<\[\\w\\W\]\+>\)`, false},
		{"CVE-2012-6708 anchored quickExpr", `/\^\(\?:\(<\[\\w\\W\]\+>\)\[\^>\]\*\|#\(\[\\w\\-\]\*\)\)\$/`, true},
		// Cross-domain text/javascript responses must not be evaluated.
		{"CVE-2015-9251 cross-domain script", `\w+\.crossDomain&&\(\w+\.contents\.script=!1\)`, true},
		// $.extend(true, {}, JSON.parse(`{"__proto__":{...}}`)) must not
		// pollute Object.prototype.
		{"CVE-2019-11358 __proto__ guard", `==="__proto__"`, true},
		// "<style><style/><img src=x onerror=...>" must not be rewritten
		// via rxhtmlTag into markup that escapes the <style> element.
		{"CVE-2020-11022 rxhtmlTag expansion", `"<\$1></\$2>"`, false},
		// "<option><style></option></select><img src=x onerror=...>" must
		// not be unconditionally wrapped in <select>.
		{"CVE-2020-11023 option wrap", `option:\[1,"<select multiple='multiple'>"`, false},
		// .load(url + " selector") must not rely on a regex to strip
		// <script> tags such as "<script>...</script >".
		{"CVE-2020-7656 load regex strip", `\.append\(\w+\.replace\(\w+,""\)\)\.find\(`, false},
		{"CVE-2020-7656 load parseHTML", `\.append\(\w+\.parseHTML\(\w+\)\)\.find\(`, true},
	} {
		if got := regexp.MustCompile(tc.re).MatchString(js); got != tc.present {
			t.Errorf("%s: match of %q in jquery.js = %v, want %v", tc.name, tc.re, got, tc.present)
		}
	}
}
