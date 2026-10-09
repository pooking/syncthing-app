// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Translation catalog. The Go side decides which language is active (system
// detection, then the user's choice) and sends the code; this file only holds
// the strings and the DOM plumbing.

const CATALOG = {
  en: {
    'app.name': 'SyncthingApp',

    // wizard
    'wizard.title': 'Welcome to SyncthingApp',
    'wizard.subtitle': 'Two quick choices and you are ready to sync.',
    'wizard.nameLabel': 'What should this device be called?',
    'wizard.namePlaceholder': 'My computer',
    'wizard.dirLabel': 'Where should shared folders live?',
    'wizard.change': 'Change…',
    'wizard.pick': 'Choose where shared folders should live',
    'wizard.done': 'Get started',
    'wizard.portable': 'Everything is stored next to the app, in {dir}',
    'wizard.standard': 'Settings and database live in {dir}',

    // tabs
    'tab.folders': 'Folders',
    'tab.devices': 'Devices',
    'tab.settings': 'Settings',
    'tab.logs': 'Logs',

    // status
    'status.starting': 'starting',
    'status.ready': 'up to date',
    'status.syncing': 'syncing',
    'status.paused': 'paused',
    'status.error': 'error',
    'status.outOfSync': 'out of sync',
    'status.needItems': '{n} item(s) to sync',
    'status.transferring': '{n} transferring',
    'status.devicesOnline': '{online}/{total} device(s) online',

    // folders
    'folders.drop': 'Drop folders here to share them',
    'folders.or': 'or',
    'folders.choose': 'choose a folder',
    'folders.empty': 'No folders shared yet. Drop one above to get started.',
    'folders.pause': 'Pause',
    'folders.resume': 'Resume',
    'folders.rescan': 'Rescan',
    'folders.open': 'Open',
    'folders.stop': 'Stop sharing',
    'folders.stopConfirm': 'Stop sharing "{name}"?\n\nThe files are kept on disk.',
    'folders.note': 'note',

    // folder states
    'folder.paused': 'paused',
    'folder.idle': 'idle',
    'folder.scanning': 'scanning',
    'folder.syncing': 'syncing',
    'folder.syncPreparing': 'preparing',
    'folder.error': 'error',
    'folder.pending': '{n} pending',
    'folder.errors': '{n} error(s)',
    'folder.inSync': 'in sync',
    // Folder states syncthing reports directly.
    'folder.sync-preparing': 'preparing',
    'folder.sync-waiting': 'waiting',
    'folder.scan-waiting': 'waiting to scan',
    'folder.cleaning': 'cleaning',

    // Message codes the backend sends with a rejected drop or a failed action.
    'code.empty-path': 'the path is empty',
    'code.path-missing': 'the path does not exist',
    'code.not-a-directory': 'that is not a folder',
    'code.cannot-resolve': 'the path could not be resolved',
    'code.filesystem-root': 'a filesystem root cannot be shared',
    'code.already-shared': 'already shared as "{name}"',
    'code.overlaps': 'overlaps with the shared folder "{name}"',
    'code.invalid-device-id': 'that is not a valid device ID',
    'code.unknown-device': 'no such device: {name}',
    'code.no-such-folder': 'no folder with ID "{name}"',
    'code.self-device': "that is this device's own ID",
    'code.name-empty': 'the name cannot be empty',
    'code.already-running': 'the app is already running',

    // devices
    'devices.thisDevice': 'This device',
    'devices.copy': 'Copy',
    'devices.copied': 'Copied',
    'devices.qr': 'QR code',
    'devices.qrAlt': 'Device ID QR code',
    'devices.thisIdHint': 'Add this ID on your other device to pair with it.',
    'devices.qrHint': 'Scan with the SyncthingApp camera on another device.',
    'devices.waiting': 'Waiting to connect',
    'devices.offered': 'Offered folders',
    'devices.offeredBy': 'offered by {name}',
    'devices.addTitle': 'Add a device',
    'devices.idPlaceholder': 'Paste the device ID (e.g. ABCDEFG-…)',
    'devices.namePlaceholder': 'Name (optional)',
    'devices.add': 'Add',
    'devices.accept': 'Accept',
    'devices.rename': 'Rename',
    'devices.renamePrompt': 'Device name',
    'devices.remove': 'Remove',
    'devices.removeConfirm': 'Unpair "{name}"?',
    'devices.connected': 'connected',
    'devices.offline': 'offline',
    'devices.paused': 'paused',

    // settings
    'settings.general': 'General',
    'settings.deviceName': 'Device name',
    'settings.autostart': 'Start SyncthingApp when I sign in',
    'settings.language': 'Language',
    'settings.languageAuto': 'Follow system',
    'settings.bandwidth': 'Bandwidth limits',
    'settings.maxSend': 'Max send rate (KB/s, 0 = unlimited)',
    'settings.maxRecv': 'Max receive rate (KB/s, 0 = unlimited)',
    'settings.connectivity': 'Connectivity',
    'settings.relays': 'Use relays when a direct connection is not possible',
    'settings.nat': 'Enable NAT traversal (UPnP)',
    'settings.sharing': 'Sharing',
    'settings.ports': 'Listening on TCP port(s) {ports}. Devices on the same network find each other automatically.',
    'settings.noPorts': 'No fixed listen port configured.',
    'settings.save': 'Save settings',
    'settings.saved': 'Saved',
    'settings.saveFailed': 'Could not save settings: {err}',
    'settings.data': 'Data',
    'settings.quit': 'Quit SyncthingApp',

    // logs
    'logs.refresh': 'Refresh',
    'logs.updated': 'Updated',
  },

  zh: {
    'app.name': 'SyncthingApp',

    'wizard.title': '欢迎使用 SyncthingApp',
    'wizard.subtitle': '两步设置，即可开始同步。',
    'wizard.nameLabel': '这台设备叫什么名字？',
    'wizard.namePlaceholder': '我的电脑',
    'wizard.dirLabel': '共享文件夹放在哪里？',
    'wizard.change': '更改…',
    'wizard.pick': '选择共享文件夹的存放位置',
    'wizard.done': '开始使用',
    'wizard.portable': '所有数据都保存在程序旁边：{dir}',
    'wizard.standard': '设置和数据库保存在：{dir}',

    'tab.folders': '文件夹',
    'tab.devices': '设备',
    'tab.settings': '设置',
    'tab.logs': '日志',

    'status.starting': '启动中',
    'status.ready': '已同步',
    'status.syncing': '同步中',
    'status.paused': '已暂停',
    'status.error': '出错',
    'status.outOfSync': '待同步',
    'status.needItems': '{n} 项待同步',
    'status.transferring': '{n} 项传输中',
    'status.devicesOnline': '{online}/{total} 台设备在线',

    'folders.drop': '把文件夹拖到这里即可共享',
    'folders.or': '或',
    'folders.choose': '选择文件夹',
    'folders.empty': '还没有共享任何文件夹，把文件夹拖到上面就能开始。',
    'folders.pause': '暂停',
    'folders.resume': '继续',
    'folders.rescan': '重新扫描',
    'folders.open': '打开',
    'folders.stop': '停止共享',
    'folders.stopConfirm': '停止共享“{name}”？\n\n磁盘上的文件会保留。',
    'folders.note': '提示',

    'folder.paused': '已暂停',
    'folder.idle': '空闲',
    'folder.scanning': '扫描中',
    'folder.syncing': '同步中',
    'folder.syncPreparing': '准备中',
    'folder.error': '出错',
    'folder.pending': '{n} 项待处理',
    'folder.errors': '{n} 个错误',
    'folder.inSync': '已同步',
    'folder.sync-preparing': '准备中',
    'folder.sync-waiting': '等待中',
    'folder.scan-waiting': '等待扫描',
    'folder.cleaning': '清理中',

    'code.empty-path': '路径为空',
    'code.path-missing': '路径不存在',
    'code.not-a-directory': '不是文件夹',
    'code.cannot-resolve': '无法解析路径',
    'code.filesystem-root': '不能共享磁盘根目录',
    'code.already-shared': '已共享为“{name}”',
    'code.overlaps': '与已共享文件夹“{name}”重叠',
    'code.invalid-device-id': '设备 ID 无效',
    'code.unknown-device': '未知设备：{name}',
    'code.no-such-folder': '没有 ID 为“{name}”的文件夹',
    'code.self-device': '这是本机自己的设备 ID',
    'code.name-empty': '名称不能为空',
    'code.already-running': '程序已在运行',

    'devices.thisDevice': '本机',
    'devices.copy': '复制',
    'devices.copied': '已复制',
    'devices.qr': '二维码',
    'devices.qrAlt': '设备 ID 二维码',
    'devices.thisIdHint': '在另一台设备上填入这个 ID 即可配对。',
    'devices.qrHint': '在另一台设备上用 SyncthingApp 扫描此二维码。',
    'devices.waiting': '等待连接',
    'devices.offered': '收到的文件夹邀请',
    'devices.offeredBy': '来自 {name}',
    'devices.addTitle': '添加设备',
    'devices.idPlaceholder': '粘贴设备 ID（如 ABCDEFG-…）',
    'devices.namePlaceholder': '名称（可选）',
    'devices.add': '添加',
    'devices.accept': '接受',
    'devices.rename': '重命名',
    'devices.renamePrompt': '设备名称',
    'devices.remove': '移除',
    'devices.removeConfirm': '解除与“{name}”的配对？',
    'devices.connected': '已连接',
    'devices.offline': '离线',
    'devices.paused': '已暂停',

    'settings.general': '常规',
    'settings.deviceName': '设备名称',
    'settings.autostart': '开机时自动启动 SyncthingApp',
    'settings.language': '界面语言',
    'settings.languageAuto': '跟随系统',
    'settings.bandwidth': '限速',
    'settings.maxSend': '最大发送速率（KB/s，0 表示不限速）',
    'settings.maxRecv': '最大接收速率（KB/s，0 表示不限速）',
    'settings.connectivity': '连接',
    'settings.relays': '无法直连时使用中继服务器',
    'settings.nat': '启用 NAT 穿透（UPnP）',
    'settings.sharing': '共享',
    'settings.ports': '正在监听 TCP 端口 {ports}。同一网络内的设备会自动互相发现。',
    'settings.noPorts': '未配置固定的监听端口。',
    'settings.save': '保存设置',
    'settings.saved': '已保存',
    'settings.saveFailed': '保存设置失败：{err}',
    'settings.data': '数据',
    'settings.quit': '退出 SyncthingApp',

    'logs.refresh': '刷新',
    'logs.updated': '已更新',
  },
};

let currentLang = 'en';

/** Set the active language; 'auto' means "whatever the backend detected". */
function setLanguage(lang) {
  currentLang = CATALOG[lang] ? lang : 'en';
  document.documentElement.lang = currentLang === 'zh' ? 'zh-CN' : 'en';
}

function currentLanguage() {
  return currentLang;
}

/**
 * Translate a key, substituting {placeholders}. A missing key falls back to
 * English and then to the key itself, so a gap shows up as text rather than
 * blank UI.
 */
function t(key, params) {
  const dict = CATALOG[currentLang] || CATALOG.en;
  let text = dict[key] ?? CATALOG.en[key] ?? key;
  if (params) {
    for (const [name, value] of Object.entries(params)) {
      text = text.split('{' + name + '}').join(String(value));
    }
  }
  return text;
}

/**
 * Replace the text of every element carrying data-i18n, plus placeholders and
 * titles. Called on load and whenever the language changes.
 */
function applyStaticText(root = document) {
  for (const el of root.querySelectorAll('[data-i18n]')) {
    el.textContent = t(el.dataset.i18n);
  }
  for (const el of root.querySelectorAll('[data-i18n-ph]')) {
    el.placeholder = t(el.dataset.i18nPh);
  }
  for (const el of root.querySelectorAll('[data-i18n-title]')) {
    el.title = t(el.dataset.i18nTitle);
  }
  for (const el of root.querySelectorAll('[data-i18n-alt]')) {
    el.alt = t(el.dataset.i18nAlt);
  }
}
