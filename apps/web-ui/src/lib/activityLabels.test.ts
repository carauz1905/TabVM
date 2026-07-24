import { describe, it, expect } from 'vitest';
import { actionLabel } from './activityLabels';

describe('actionLabel', () => {
  it('maps known action codes to short English labels', () => {
    expect(actionLabel('vm.start')).toBe('Start VM');
    expect(actionLabel('vm.stop')).toBe('Stop VM (ACPI)');
    expect(actionLabel('vm.poweroff')).toBe('Force power off');
    expect(actionLabel('guest-additions.install')).toBe('Install Guest Additions');
    expect(actionLabel('network.forwarding.add')).toBe('Add port forwarding rule');
    expect(actionLabel('vm.usb.attach')).toBe('Attach USB device');
    expect(actionLabel('snapshot.take')).toBe('Take snapshot');
    expect(actionLabel('vm.create.cleanup')).toBe('Clean up failed VM create');
    expect(actionLabel('sharedfolder.remove')).toBe('Remove shared folder');
  });

  it('falls back to the raw code for unknown actions', () => {
    expect(actionLabel('totally.unknown')).toBe('totally.unknown');
    expect(actionLabel('')).toBe('');
  });
});
