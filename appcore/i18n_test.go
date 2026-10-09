// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"errors"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	cases := map[string]Lang{
		"zh": LangChinese, "zh-CN": LangChinese, "zh_CN": LangChinese,
		"zh-Hans": LangChinese, "ZH-TW": LangChinese,
		"en": LangEnglish, "en-US": LangEnglish, "en_GB": LangEnglish,
		"fr": "", "": "", "de-DE": "",
	}
	for in, want := range cases {
		if got := NormalizeLang(in); got != want {
			t.Errorf("NormalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripEncoding(t *testing.T) {
	cases := map[string]string{
		"zh_CN.UTF-8": "zh-CN",
		"en_GB@euro":  "en-GB",
		"C":           "C",
		"zh_CN":       "zh-CN",
	}
	for in, want := range cases {
		if got := stripEncoding(in); got != want {
			t.Errorf("stripEncoding(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranslate(t *testing.T) {
	if got := Translate(LangChinese, CodeAlreadyShared, "photos"); got != "已共享为“photos”" {
		t.Errorf("Chinese translation = %q", got)
	}
	if got := Translate(LangEnglish, CodeAlreadyShared, "photos"); got != `already shared as "photos"` {
		t.Errorf("English translation = %q", got)
	}
	// An unknown language falls back to English rather than rendering nothing.
	if got := Translate(Lang("fr"), CodeSelfDevice); got != "that is this device's own ID" {
		t.Errorf("fallback translation = %q", got)
	}
}

// TestCatalogsAreComplete guards against a translation being added to one
// language and forgotten in the other.
func TestCatalogsAreComplete(t *testing.T) {
	for code := range messages[LangEnglish] {
		if _, ok := messages[LangChinese][code]; !ok {
			t.Errorf("code %q has no Chinese translation", code)
		}
	}
	for code := range messages[LangChinese] {
		if _, ok := messages[LangEnglish][code]; !ok {
			t.Errorf("code %q has no English translation", code)
		}
	}
}

func TestUserErrorMatchesByCode(t *testing.T) {
	if errors.Is(NewUserError(CodeNotADirectory), ErrSelfDevice) {
		t.Error("unrelated codes must not compare equal")
	}
	if !errors.Is(NewUserError(CodeSelfDevice), ErrSelfDevice) {
		t.Error("the same code should compare equal")
	}
	if errors.Is(NewUserError(CodeAlreadyRunning), ErrDuplicateFolder) {
		t.Error("unrelated codes must not compare equal")
	}
}

// TestTranslateAllCodes ensures every code renders in both languages, which
// catches a mis-numbered format verb in a translation.
func TestTranslateAllCodes(t *testing.T) {
	for _, lang := range SupportedLangs {
		for code, format := range messages[LangEnglish] {
			out := Translate(lang, code, "x", 1, "y")
			if out == "" {
				t.Errorf("Translate(%q, %q) returned an empty string", lang, code)
			}
			if out == string(code) {
				t.Errorf("Translate(%q, %q) fell back to the raw code (format %q)", lang, code, format)
			}
		}
	}
}
