import { describe, it, expect, beforeEach, vi } from 'vitest';
import * as store from './layoutStore.svelte';

// Mock localStorage
const localStorageMock = (() => {
	let storage: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => storage[key] ?? null),
		setItem: vi.fn((key: string, value: string) => {
			storage[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete storage[key];
		}),
		clear: vi.fn(() => {
			storage = {};
		}),
		get length() {
			return Object.keys(storage).length;
		},
		key: vi.fn((_: number) => null)
	};
})();

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock });

describe('layoutStore', () => {
	beforeEach(() => {
		store._resetForTesting();
		localStorageMock.clear();
		vi.clearAllMocks();
	});

	describe('default state', () => {
		it('has a single leaf root node with panelType services', () => {
			const root = store.getRootNode();
			expect(root.type).toBe('leaf');
			if (root.type === 'leaf') {
				expect(root.panelType).toBe('services');
				expect(root.panelId).toBeTruthy();
			}
		});

		it('has activePanelId set to the root leaf panelId', () => {
			const root = store.getRootNode();
			if (root.type === 'leaf') {
				expect(store.getActivePanelId()).toBe(root.panelId);
			}
		});
	});

	describe('splitPanel', () => {
		it('creates a branch with two leaves at 50/50', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const originalId = root.panelId;

			store.splitPanel(originalId, 'horizontal');

			const newRoot = store.getRootNode();
			expect(newRoot.type).toBe('branch');
			if (newRoot.type === 'branch') {
				expect(newRoot.ratio).toBe(0.5);
				expect(newRoot.first.type).toBe('leaf');
				expect(newRoot.second.type).toBe('leaf');
			}
		});

		it('horizontal split creates direction horizontal', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');

			const newRoot = store.getRootNode();
			if (newRoot.type === 'branch') {
				expect(newRoot.direction).toBe('horizontal');
			}
		});

		it('vertical split creates direction vertical', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'vertical');

			const newRoot = store.getRootNode();
			if (newRoot.type === 'branch') {
				expect(newRoot.direction).toBe('vertical');
			}
		});

		it('original leaf retains its panelType', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const originalId = root.panelId;

			store.splitPanel(originalId, 'horizontal');

			const newRoot = store.getRootNode();
			if (newRoot.type === 'branch' && newRoot.first.type === 'leaf') {
				expect(newRoot.first.panelId).toBe(originalId);
				expect(newRoot.first.panelType).toBe('services');
			}
		});

		it('new leaf gets panelType services by default', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');

			const newRoot = store.getRootNode();
			if (newRoot.type === 'branch' && newRoot.second.type === 'leaf') {
				expect(newRoot.second.panelType).toBe('services');
			}
		});
	});

	describe('closePanel', () => {
		it('on 2-panel layout promotes sibling to root', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const secondId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';

			store.closePanel(secondId);

			const afterClose = store.getRootNode();
			expect(afterClose.type).toBe('leaf');
			if (afterClose.type === 'leaf') {
				expect(afterClose.panelId).toBe(root.panelId);
			}
		});

		it('on last panel is no-op', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.closePanel(root.panelId);

			expect(store.getRootNode().type).toBe('leaf');
		});

		it('updates activePanelId to sibling when active panel is closed', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const originalId = root.panelId;

			store.splitPanel(originalId, 'horizontal');
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const secondId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';

			store.setActivePanel(secondId);
			store.closePanel(secondId);

			expect(store.getActivePanelId()).toBe(originalId);
		});
	});

	describe('resizePanel', () => {
		it('updates ratio on parent branch', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const firstId =
				branch.first.type === 'leaf' ? branch.first.panelId : '';

			store.resizePanel(firstId, 0.7);

			const updated = store.getRootNode();
			if (updated.type === 'branch') {
				expect(updated.ratio).toBe(0.7);
			}
		});

		it('clamps ratio to minimum 0.1', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const firstId =
				branch.first.type === 'leaf' ? branch.first.panelId : '';

			store.resizePanel(firstId, 0.01);

			const updated = store.getRootNode();
			if (updated.type === 'branch') {
				expect(updated.ratio).toBe(0.1);
			}
		});

		it('clamps ratio to maximum 0.9', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const firstId =
				branch.first.type === 'leaf' ? branch.first.panelId : '';

			store.resizePanel(firstId, 0.99);

			const updated = store.getRootNode();
			if (updated.type === 'branch') {
				expect(updated.ratio).toBe(0.9);
			}
		});
	});

	describe('setActivePanel', () => {
		it('updates activePanelId', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const secondId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';

			store.setActivePanel(secondId);
			expect(store.getActivePanelId()).toBe(secondId);
		});
	});

	describe('setPanelType', () => {
		it('updates leaf panelType', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.setPanelType(root.panelId, 'terminal');

			const updated = store.getRootNode();
			if (updated.type === 'leaf') {
				expect(updated.panelType).toBe('terminal');
			}
		});

		it('on non-existent panelId is no-op', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.setPanelType('nonexistent', 'terminal');

			const updated = store.getRootNode();
			if (updated.type === 'leaf') {
				expect(updated.panelType).toBe('services');
			}
		});
	});

	describe('isLastPanel', () => {
		it('returns true for single-leaf root', () => {
			expect(store.getIsLastPanel()).toBe(true);
		});

		it('returns false after split', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');
			expect(store.getIsLastPanel()).toBe(false);
		});
	});

	describe('_resetForTesting', () => {
		it('restores default state', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.splitPanel(root.panelId, 'horizontal');
			store._resetForTesting();

			const reset = store.getRootNode();
			expect(reset.type).toBe('leaf');
			if (reset.type === 'leaf') {
				expect(reset.panelType).toBe('services');
			}
		});
	});

	describe('focusAdjacentPanel', () => {
		it('focus right from left panel in horizontal split moves to right panel', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const leftId = root.panelId;

			store.splitPanel(leftId, 'horizontal');
			store.setActivePanel(leftId);

			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const rightId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';

			store.focusAdjacentPanel('right');
			expect(store.getActivePanelId()).toBe(rightId);
		});

		it('focus left from right panel moves to left panel', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const leftId = root.panelId;

			store.splitPanel(leftId, 'horizontal');
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const rightId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';

			store.setActivePanel(rightId);
			store.focusAdjacentPanel('left');
			expect(store.getActivePanelId()).toBe(leftId);
		});

		it('focus down from top panel in vertical split moves to bottom panel', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const topId = root.panelId;

			store.splitPanel(topId, 'vertical');
			store.setActivePanel(topId);

			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const bottomId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';

			store.focusAdjacentPanel('down');
			expect(store.getActivePanelId()).toBe(bottomId);
		});

		it('focus in direction with no adjacent panel is no-op', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const leftId = root.panelId;

			store.splitPanel(leftId, 'horizontal');
			store.setActivePanel(leftId);

			store.focusAdjacentPanel('left');
			expect(store.getActivePanelId()).toBe(leftId);
		});

		it('focus navigation in nested splits finds correct leaf', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const firstId = root.panelId;

			// Split horizontally: [first | second]
			store.splitPanel(firstId, 'horizontal');
			let branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const secondId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';

			// Split second vertically: [first | [second-top / second-bottom]]
			store.splitPanel(secondId, 'vertical');

			// Navigate from first to right should land on the first leaf in the right subtree
			store.setActivePanel(firstId);
			store.focusAdjacentPanel('right');

			// Should be in the right subtree now
			expect(store.getActivePanelId()).not.toBe(firstId);
		});
	});

	describe('presets', () => {
		it('savePreset adds entry to presets map', () => {
			store.savePreset('Test Preset');
			expect(store.getPresets().has('Test Preset')).toBe(true);
		});

		it('savePreset writes to localStorage', () => {
			store.savePreset('Test Preset');
			expect(localStorageMock.setItem).toHaveBeenCalled();
		});

		it('restorePreset sets rootNode to saved tree', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			store.splitPanel(root.panelId, 'horizontal');
			store.savePreset('Split Layout');

			// Close a panel to get back to single leaf, then restore
			const branch = store.getRootNode();
			if (branch.type !== 'branch') throw new Error('expected branch');
			const secondId =
				branch.second.type === 'leaf' ? branch.second.panelId : '';
			store.closePanel(secondId);
			expect(store.getRootNode().type).toBe('leaf');

			store.restorePreset('Split Layout');
			expect(store.getRootNode().type).toBe('branch');
		});

		it('restorePreset regenerates panelIds', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');
			const originalId = root.panelId;

			store.savePreset('Original');
			store.restorePreset('Original');

			const restored = store.getRootNode();
			if (restored.type === 'leaf') {
				expect(restored.panelId).not.toBe(originalId);
			}
		});

		it('restorePreset with unavailable panelType preserves type', () => {
			const root = store.getRootNode();
			if (root.type !== 'leaf') throw new Error('expected leaf');

			store.setPanelType(root.panelId, 'terminal');
			store.savePreset('Terminal');

			// Change back to services
			store.setPanelType(
				(store.getRootNode() as { panelId: string }).panelId,
				'services'
			);

			store.restorePreset('Terminal');

			const restored = store.getRootNode();
			if (restored.type === 'leaf') {
				expect(restored.panelType).toBe('terminal');
			}
		});

		it('deletePreset removes from map', () => {
			store.savePreset('ToDelete');
			expect(store.getPresets().has('ToDelete')).toBe(true);
			store.deletePreset('ToDelete');
			expect(store.getPresets().has('ToDelete')).toBe(false);
		});

		it('deletePreset updates localStorage', () => {
			store.savePreset('ToDelete');
			localStorageMock.setItem.mockClear();
			store.deletePreset('ToDelete');
			expect(localStorageMock.setItem).toHaveBeenCalled();
		});

		it('loadPresetsFromStorage with empty localStorage seeds Monitoring default', () => {
			store.loadPresetsFromStorage();
			expect(store.getPresets().has('Monitoring')).toBe(true);
		});

		it('loadPresetsFromStorage with valid JSON restores presets', () => {
			const data = {
				'My Preset': { type: 'leaf', panelType: 'services', panelId: 'p1' }
			};
			localStorageMock.getItem.mockReturnValueOnce(JSON.stringify(data));
			store.loadPresetsFromStorage();
			expect(store.getPresets().has('My Preset')).toBe(true);
		});

		it('loadPresetsFromStorage with corrupt JSON falls back to default', () => {
			localStorageMock.getItem.mockReturnValueOnce('{{invalid json');
			store.loadPresetsFromStorage();
			expect(store.getPresets().has('Monitoring')).toBe(true);
		});

		it('saveActiveLayout persists current tree', () => {
			store.saveActiveLayout();
			expect(localStorageMock.setItem).toHaveBeenCalledWith(
				'command-center:active-layout',
				expect.any(String)
			);
		});

		it('loadActiveLayout restores persisted tree', () => {
			const tree = { type: 'leaf', panelType: 'services', panelId: 'saved-id' };
			localStorageMock.getItem.mockReturnValueOnce(JSON.stringify(tree));
			const result = store.loadActiveLayout();
			expect(result).toBe(true);
			expect(store.getRootNode().type).toBe('leaf');
		});

		it('_resetForTesting resets preset state', () => {
			store.savePreset('Test');
			store._resetForTesting();
			expect(store.getPresets().size).toBe(0);
			expect(store.getActivePresetName()).toBe(null);
		});
	});
});
