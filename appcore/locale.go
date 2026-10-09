// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"os"
	"strings"
)

// DetectSystemLanguage picks a language from the OS locale, falling back to
// English when nothing matches.
//
// The POSIX variables are checked first because they are set on Linux and are
// usually honored on macOS and in a launched-from-terminal app on Windows.
// platformLanguage covers the cases where they are absent.
func DetectSystemLanguage() Lang {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			if lang := NormalizeLang(stripEncoding(v)); lang != "" {
				return lang
			}
		}
	}
	if v := platformLanguage(); v != "" {
		if lang := NormalizeLang(v); lang != "" {
			return lang
		}
	}
	return DefaultLang
}

// stripEncoding turns "zh_CN.UTF-8" into "zh_CN" and "en_GB@euro" into "en_GB".
func stripEncoding(v string) string {
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	return strings.ReplaceAll(v, "_", "-")
}
