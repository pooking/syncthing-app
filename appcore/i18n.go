// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Lang is a supported interface language.
type Lang string

const (
	LangEnglish Lang = "en"
	LangChinese Lang = "zh"
)

// DefaultLang is used when nothing better can be determined.
const DefaultLang = LangEnglish

// SupportedLangs lists the languages the UI and the messages below are
// translated into.
var SupportedLangs = []Lang{LangEnglish, LangChinese}

// Code identifies a translatable message. Codes are stable strings so the
// frontend can translate the same message independently.
type Code string

const (
	CodeEmptyPath       Code = "empty-path"
	CodePathMissing     Code = "path-missing"
	CodeNotADirectory   Code = "not-a-directory"
	CodeCannotResolve   Code = "cannot-resolve"
	CodeFilesystemRoot  Code = "filesystem-root"
	CodeAlreadyShared   Code = "already-shared"
	CodeOverlaps        Code = "overlaps"
	CodeInvalidDeviceID Code = "invalid-device-id"
	CodeUnknownDevice   Code = "unknown-device"
	CodeNoSuchFolder    Code = "no-such-folder"
	CodeSelfDevice      Code = "self-device"
	CodeNameEmpty       Code = "name-empty"
	CodeAlreadyRunning  Code = "already-running"

	CodeCreateDataDir   Code = "create-data-dir"
	CodeAcquireLock     Code = "acquire-lock"
	CodeCertificate     Code = "certificate"
	CodeLoadConfig      Code = "load-config"
	CodeApplyConfig     Code = "apply-config"
	CodeOpenDatabase    Code = "open-database"
	CodeCreateApp       Code = "create-app"
	CodeStartApp        Code = "start-app"
	CodeOpenLog         Code = "open-log"
	CodeSaveConfig      Code = "save-config"
	CodeRemoveDirectory Code = "remove-directory"
	CodeCreateSyncDir   Code = "create-sync-dir"
	CodeWriteSetup      Code = "write-setup"
	CodeAutostart       Code = "autostart"

	// Tray menu and tooltip.
	CodeTrayOpen       Code = "tray-open"
	CodeTrayPause      Code = "tray-pause"
	CodeTrayDevice     Code = "tray-device"
	CodeTrayQuit       Code = "tray-quit"
	CodeTrayStarting   Code = "tray-starting"
	CodeTrayUpToDate   Code = "tray-up-to-date"
	CodeTraySyncing    Code = "tray-syncing"
	CodeTraySyncingPct Code = "tray-syncing-pct"
	CodeTrayPaused     Code = "tray-paused"
	CodeTrayProblems   Code = "tray-problems"
	CodeTrayNoDevices  Code = "tray-no-devices"
	CodeTrayNeedItems  Code = "tray-need-items"

	// Native dialog titles.
	CodeDialogPickFolder Code = "dialog-pick-folder"
)

// messages holds every translatable string. A code missing from a language
// falls back to English.
var messages = map[Lang]map[Code]string{
	LangEnglish: {
		CodeEmptyPath:       "empty path",
		CodePathMissing:     "path does not exist",
		CodeNotADirectory:   "not a directory",
		CodeCannotResolve:   "cannot resolve path",
		CodeFilesystemRoot:  "cannot share a filesystem root",
		CodeAlreadyShared:   "already shared as %q",
		CodeOverlaps:        "overlaps with shared folder %q",
		CodeInvalidDeviceID: "invalid device ID: %v",
		CodeUnknownDevice:   "unknown device %q",
		CodeNoSuchFolder:    "no folder with ID %q",
		CodeSelfDevice:      "that is this device's own ID",
		CodeNameEmpty:       "device name cannot be empty",
		CodeAlreadyRunning:  "another instance is already running",

		CodeCreateDataDir:   "create data directory: %v",
		CodeAcquireLock:     "acquire lock: %v",
		CodeCertificate:     "load or generate certificate: %v",
		CodeLoadConfig:      "load configuration: %v",
		CodeApplyConfig:     "apply configuration: %v",
		CodeOpenDatabase:    "open database: %v",
		CodeCreateApp:       "create app: %v",
		CodeStartApp:        "start app: %v",
		CodeOpenLog:         "open log file: %v",
		CodeSaveConfig:      "save configuration: %v",
		CodeRemoveDirectory: "remove directory: %v",
		CodeCreateSyncDir:   "create sync directory: %v",
		CodeWriteSetup:      "write setup marker: %v",
		CodeAutostart:       "set start at login: %v",

		CodeTrayOpen:       "Open SyncthingApp",
		CodeTrayPause:      "Pause syncing",
		CodeTrayDevice:     "Device ID: %s",
		CodeTrayQuit:       "Quit",
		CodeTrayStarting:   "SyncthingApp — starting",
		CodeTrayUpToDate:   "SyncthingApp — up to date",
		CodeTraySyncing:    "SyncthingApp — syncing",
		CodeTraySyncingPct: "SyncthingApp — syncing %d%%",
		CodeTrayPaused:     "SyncthingApp — paused",
		CodeTrayProblems:   "SyncthingApp — %d problem(s)",
		CodeTrayNoDevices:  "SyncthingApp — no devices connected",
		CodeTrayNeedItems:  "SyncthingApp — %d item(s) to sync",

		CodeDialogPickFolder: "Choose a folder",
	},
	LangChinese: {
		CodeEmptyPath:       "路径为空",
		CodePathMissing:     "路径不存在",
		CodeNotADirectory:   "不是文件夹",
		CodeCannotResolve:   "无法解析路径",
		CodeFilesystemRoot:  "不能共享磁盘根目录",
		CodeAlreadyShared:   "已共享为“%s”",
		CodeOverlaps:        "与已共享文件夹“%s”重叠",
		CodeInvalidDeviceID: "设备 ID 无效：%v",
		CodeUnknownDevice:   "未知设备“%s”",
		CodeNoSuchFolder:    "没有 ID 为“%s”的文件夹",
		CodeSelfDevice:      "这是本机自己的设备 ID",
		CodeNameEmpty:       "设备名称不能为空",
		CodeAlreadyRunning:  "已有一个实例在运行",

		CodeCreateDataDir:   "创建数据目录失败：%v",
		CodeAcquireLock:     "获取文件锁失败：%v",
		CodeCertificate:     "加载或生成证书失败：%v",
		CodeLoadConfig:      "加载配置失败：%v",
		CodeApplyConfig:     "应用配置失败：%v",
		CodeOpenDatabase:    "打开数据库失败：%v",
		CodeCreateApp:       "创建实例失败：%v",
		CodeStartApp:        "启动失败：%v",
		CodeOpenLog:         "打开日志文件失败：%v",
		CodeSaveConfig:      "保存配置失败：%v",
		CodeRemoveDirectory: "删除目录失败：%v",
		CodeCreateSyncDir:   "创建同步目录失败：%v",
		CodeWriteSetup:      "写入初始化标记失败：%v",
		CodeAutostart:       "设置开机自启失败：%v",

		CodeTrayOpen:       "打开 SyncthingApp",
		CodeTrayPause:      "暂停同步",
		CodeTrayDevice:     "设备 ID：%s",
		CodeTrayQuit:       "退出",
		CodeTrayStarting:   "SyncthingApp — 启动中",
		CodeTrayUpToDate:   "SyncthingApp — 已同步",
		CodeTraySyncing:    "SyncthingApp — 同步中",
		CodeTraySyncingPct: "SyncthingApp — 同步中 %d%%",
		CodeTrayPaused:     "SyncthingApp — 已暂停",
		CodeTrayProblems:   "SyncthingApp — %d 个问题",
		CodeTrayNoDevices:  "SyncthingApp — 没有设备连接",
		CodeTrayNeedItems:  "SyncthingApp — %d 项待同步",

		CodeDialogPickFolder: "选择文件夹",
	},
}

// Translate renders a message code in the given language, falling back to
// English and then to the raw code.
func Translate(lang Lang, code Code, args ...any) string {
	format, ok := messages[lang][code]
	if !ok {
		format, ok = messages[LangEnglish][code]
	}
	if !ok {
		return string(code)
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// NormalizeLang maps a language tag such as "zh-CN", "zh_Hans" or "en-US" onto
// a supported language.
func NormalizeLang(tag string) Lang {
	tag = strings.ToLower(strings.TrimSpace(tag))
	switch {
	case strings.HasPrefix(tag, "zh"):
		return LangChinese
	case strings.HasPrefix(tag, "en"):
		return LangEnglish
	default:
		return ""
	}
}

// UserError is an error carrying a message code so the UI can show it in the
// user's language. Two UserErrors match under errors.Is when their codes
// match, which lets sentinel errors stay comparable.
type UserError struct {
	Code Code
	Args []any
}

func (e *UserError) Error() string {
	return Translate(DefaultLang, e.Code, e.Args...)
}

// Is reports whether target carries the same message code.
func (e *UserError) Is(target error) bool {
	var other *UserError
	if errors.As(target, &other) {
		return other.Code == e.Code
	}
	return false
}

// NewUserError builds an error with a message code.
func NewUserError(code Code, args ...any) *UserError {
	return &UserError{Code: code, Args: args}
}

// errf builds an error whose text is rendered in the app's current language.
func (a *App) errf(code Code, args ...any) error {
	return &localizedError{code: code, args: args, lang: a.Language()}
}

// localizedError is a UserError that renders in a specific language rather than
// the default one.
type localizedError struct {
	code Code
	args []any
	lang Lang
}

func (e *localizedError) Error() string { return Translate(e.lang, e.code, e.args...) }

func (e *localizedError) Is(target error) bool {
	var other *UserError
	if errors.As(target, &other) {
		return other.Code == e.code
	}
	var otherLocalized *localizedError
	if errors.As(target, &otherLocalized) {
		return otherLocalized.code == e.code
	}
	return false
}

// errors sent to callers that only need to distinguish the cause.
var (
	// ErrAlreadyRunning is returned by Start when another instance holds the
	// data directory lock.
	ErrAlreadyRunning = &UserError{Code: CodeAlreadyRunning}
	// ErrSelfDevice is returned when the user tries to add their own device ID.
	ErrSelfDevice = &UserError{Code: CodeSelfDevice}
	// ErrDuplicateFolder is returned when the directory is already shared.
	ErrDuplicateFolder = &UserError{Code: CodeAlreadyShared}
)

// prefs is the app's own settings, kept apart from syncthing's config.xml.
type prefs struct {
	Language Lang `json:"language,omitempty"`
}

func (a *App) prefsPath() string { return filepath.Join(a.dataDir, "prefs.json") }

// loadPrefs reads the app's own settings, ignoring a missing or unreadable
// file: defaults are always usable.
func (a *App) loadPrefs() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lang = DetectSystemLanguage()

	data, err := os.ReadFile(a.prefsPath())
	if err != nil {
		return
	}
	var p prefs
	if err := json.Unmarshal(data, &p); err != nil {
		return
	}
	if lang := NormalizeLang(string(p.Language)); lang != "" {
		a.lang = lang
	}
}

func (a *App) savePrefs() error {
	a.mu.Lock()
	p := prefs{Language: a.lang}
	a.mu.Unlock()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.prefsPath(), append(data, '\n'), 0o600)
}

// Language returns the language the UI and messages use.
func (a *App) Language() Lang {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lang == "" {
		return DefaultLang
	}
	return a.lang
}

// SetLanguage changes the interface language and persists the choice.
func (a *App) SetLanguage(lang Lang) error {
	if NormalizeLang(string(lang)) == "" {
		return fmt.Errorf("unsupported language %q", lang)
	}
	a.mu.Lock()
	a.lang = NormalizeLang(string(lang))
	a.mu.Unlock()
	return a.savePrefs()
}

// T renders a message code in the current language. The frontend uses the same
// codes through its own catalog.
func (a *App) T(code Code, args ...any) string {
	return Translate(a.Language(), code, args...)
}
