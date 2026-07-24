// Maps backend activity action codes (e.g. "vm.start") to short, human-readable
// English labels. The English label doubles as the i18n dictionary key, so the
// Activity view renders t(actionLabel(code)) to get the localized text. Unknown
// codes fall back to the raw code unchanged so new backend actions never crash
// or hide a row.
const ACTION_LABELS: Record<string, string> = {
  'clipboard.set': 'Set clipboard mode',
  'console.disable': 'Disable console',
  'disk.add': 'Add disk',
  'disk.detach': 'Detach disk',
  'disk.resize': 'Resize disk',
  'file.transfer': 'Transfer file to guest',
  'guest-additions.install': 'Install Guest Additions',
  'guest-additions.update': 'Update Guest Additions',
  'network.forwarding.add': 'Add port forwarding rule',
  'network.forwarding.delete': 'Delete port forwarding rule',
  'network.link': 'Set network cable',
  'network.mode': 'Change network mode',
  'serial.getty': 'Configure serial getty',
  'sharedfolder.add': 'Add shared folder',
  'sharedfolder.remove': 'Remove shared folder',
  'snapshot.delete': 'Delete snapshot',
  'snapshot.restore': 'Restore snapshot',
  'snapshot.take': 'Take snapshot',
  'vm.clone': 'Clone VM',
  'vm.create': 'Create VM',
  'vm.create.cleanup': 'Clean up failed VM create',
  'vm.delete': 'Delete VM',
  'vm.dvd.eject': 'Eject ISO',
  'vm.dvd.mount': 'Mount ISO',
  'vm.export': 'Export VM',
  'vm.guest.copyfrom': 'Copy file from guest',
  'vm.guest.run': 'Run command in guest',
  'vm.hardware': 'Change hardware',
  'vm.import': 'Import VM',
  'vm.savestate': 'Suspend VM',
  'vm.serial.disable': 'Disable serial terminal',
  'vm.serial.enable': 'Enable serial terminal',
  'vm.start': 'Start VM',
  'vm.stop': 'Stop VM (ACPI)',
  'vm.poweroff': 'Force power off',
  'vm.reset': 'Reset VM',
  'vm.usb.attach': 'Attach USB device',
  'vm.usb.detach': 'Detach USB device',
};

// actionLabel returns the English human label for a known backend action code,
// or the raw code itself when the code is not recognized.
export function actionLabel(action: string): string {
  return ACTION_LABELS[action] ?? action;
}
