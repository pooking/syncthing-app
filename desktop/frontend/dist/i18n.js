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
    'app.name': 'OspreySyncTool',

    // startup splash
    'boot.title': 'Starting OspreySyncTool…',
    'boot.detail': 'Loading the configuration and database.',
    'boot.slow': 'This is taking longer than usual. The app is still working; you can look at the window meanwhile.',
    'boot.showAnyway': 'Show the window anyway',

    // wizard
    'wizard.title': 'Welcome to OspreySyncTool',
    'wizard.subtitle': 'Two quick choices and you are ready to sync.',
    'wizard.nameLabel': 'What should this device be called?',
    'wizard.namePlaceholder': 'My computer',
    'wizard.dirLabel': 'Where should shared folders live?',
    'wizard.dirPlaceholder': 'C:\\Users\\you\\Sync',
    'wizard.dirHint': 'Type a path, or use Change… to pick one. It is created if it does not exist.',
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
    'status.reload': 'Reload',
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
    'folders.move': 'Move…',
    'folders.movePick': 'Choose the new location for "{name}"',
    'folders.moveConfirm': 'Move "{name}" to:\n\n{target}\n\n{files} file(s), {size}. The files are copied and then removed from the old location. Shared folders are not affected on other devices.',
    'folders.moveMergeConfirm': 'Move "{name}" to:\n\n{target}\n\nThat directory already has files in it. They will be merged with the folder\'s own files.\n\n{files} file(s), {size}. The files are copied and then removed from the old location.',
    'folders.moveReplaceConfirm': 'There are files in {target} that are not part of "{name}". Delete them? Choose No to keep them.',
    'folders.moveFailed': 'Could not move the folder: {err}',

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
    'devices.qrHint': 'Scan with the OspreySyncTool camera on another device.',
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
    'settings.deviceNameHint': 'Announced to other devices when they connect. A device that already knows this one keeps the name it has until it is removed and re-added there.',
    'settings.autostart': 'Start OspreySyncTool when I sign in',
    'settings.language': 'Language',
    'settings.languageAuto': 'Follow system',
    'settings.bandwidth': 'Bandwidth limits',
    'settings.maxSend': 'Max send rate (KB/s, 0 = unlimited)',
    'settings.maxRecv': 'Max receive rate (KB/s, 0 = unlimited)',
    'settings.connectivity': 'Connectivity',
    'settings.relays': 'Use relays when a direct connection is not possible',
    'settings.nat': 'Enable NAT traversal (UPnP)',
    'settings.hub': 'Hub device',
    'settings.hubLine': '{name} ({id}) at {address}. Built in: it connects automatically and needs no pairing.',
    'settings.hubShare': 'Also copy shared folders to the hub, as a backup',
    'settings.hubShareHint': 'With this on, a new machine can fetch a folder from the hub alone, without any other computer being online. It costs space on the hub. Turning it off stops future folders from joining; folders already syncing there keep doing so.',
    'settings.hubShared': 'Hub sharing on',
    'settings.hubNotShared': 'Hub sharing off',
    'settings.sharing': 'Sharing',
    'settings.ports': 'Listening on TCP port(s) {ports}. Devices on the same network find each other automatically.',
    'settings.noPorts': 'No fixed listen port configured.',
    'settings.save': 'Save settings',
    'settings.saved': 'Saved',
    'settings.saveFailed': 'Could not save settings: {err}',
    'settings.data': 'Data',    'settings.dataDirLabel': 'Settings and database are stored here',
    'settings.dataDirMove': 'Move…',
    'settings.dataDirPick': 'Choose the new data directory',
    'settings.dataDirConfirm': 'Copy the settings and database to {dir} and restart?\n\nShared folders themselves are NOT moved, and the old directory is left behind as a backup. The app closes and starts again.',
    'settings.dataDirFailed': 'Could not change the data directory: {err}',
    'settings.quit': 'Quit OspreySyncTool',

    // logs
    'logs.refresh': 'Refresh',
    'logs.updated': 'Updated',
  },

  zh: {
    'app.name': 'OspreySyncTool',

    'boot.title': '正在启动 OspreySyncTool…',
    'boot.detail': '正在加载配置和数据库。',
    'boot.slow': '比平常慢一些。程序还在启动，你可以先看看界面。',
    'boot.showAnyway': '仍然显示界面',

    'wizard.title': '欢迎使用 OspreySyncTool',
    'wizard.subtitle': '两步设置，即可开始同步。',
    'wizard.nameLabel': '这台设备叫什么名字？',
    'wizard.namePlaceholder': '我的电脑',
    'wizard.dirLabel': '共享文件夹放在哪里？',
    'wizard.dirPlaceholder': 'D:\\同步',
    'wizard.dirHint': '可以直接输入路径，也可以点「更改…」选择。目录不存在会自动创建。',
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
    'status.reload': '重新加载',
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
    'folders.move': '移动…',
    'folders.movePick': '为“{name}”选择新位置',
    'folders.moveConfirm': '把“{name}”移动到：\n\n{target}\n\n{files} 个文件，{size}。文件会被复制过去，然后从原位置删除。其他设备上的共享不受影响。',
    'folders.moveMergeConfirm': '把“{name}”移动到：\n\n{target}\n\n该目录里已经有文件，会与这个文件夹的内容合并。\n\n{files} 个文件，{size}。文件会被复制过去，然后从原位置删除。',
    'folders.moveReplaceConfirm': '合并后，删除 {target} 中不属于“{name}”的文件？选“否”则保留。',
    'folders.moveFailed': '移动文件夹失败：{err}',

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
    'devices.qrHint': '在另一台设备上用 OspreySyncTool 扫描此二维码。',
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
    'settings.deviceNameHint': '对方设备连接时会收到这个名字。已经知道本机的设备会保留它原来记下的名字，除非在那边删除后重新添加。',
    'settings.autostart': '开机时自动启动 OspreySyncTool',
    'settings.language': '界面语言',
    'settings.languageAuto': '跟随系统',
    'settings.bandwidth': '限速',
    'settings.maxSend': '最大发送速率（KB/s，0 表示不限速）',
    'settings.maxRecv': '最大接收速率（KB/s，0 表示不限速）',
    'settings.connectivity': '连接',
    'settings.relays': '无法直连时使用中继服务器',
    'settings.nat': '启用 NAT 穿透（UPnP）',
    'settings.hub': '中转站',
    'settings.hubLine': '{name}（{id}），地址 {address}。已内置于程序，自动连接，无需手动添加。',
    'settings.hubShare': '同时把共享文件夹复制到中转站作为备份',
    'settings.hubShareHint': '打开后，新机器可以只从中转站取文件夹，不需要其他电脑在线。代价是占用中转站的磁盘空间。关掉之后仅影响以后新建的文件夹；已经在往中转站同步的文件夹会保持原样。',
    'settings.hubShared': '已开启中转站备份',
    'settings.hubNotShared': '已关闭中转站备份',
    'settings.sharing': '共享',
    'settings.ports': '正在监听 TCP 端口 {ports}。同一网络内的设备会自动互相发现。',
    'settings.noPorts': '未配置固定的监听端口。',
    'settings.save': '保存设置',
    'settings.saved': '已保存',
    'settings.saveFailed': '保存设置失败：{err}',
    'settings.data': '数据',
    'settings.dataDirLabel': '设置和数据库保存在这里',
    'settings.dataDirMove': '更改…',
    'settings.dataDirPick': '选择新的数据目录',
    'settings.dataDirConfirm': '把设置和数据库复制到 {dir} 并重启？\n\n共享的文件夹本身不会移动，旧目录会保留作为备份。程序会关闭后自动重新启动。',
    'settings.dataDirFailed': '无法更改数据目录：{err}',
    'settings.quit': '退出 OspreySyncTool',

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
