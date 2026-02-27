import type { LayoutNode, LeafNode, BranchNode, PanelType } from './types';

export const AVAILABLE_PANEL_TYPES: Set<PanelType> = new Set(['services']);

function generatePanelId(): string {
	return crypto.randomUUID();
}

function createDefaultLeaf(): LeafNode {
	return { type: 'leaf', panelType: 'services', panelId: generatePanelId() };
}

// Internal reactive state
const _defaultLeaf = createDefaultLeaf();
let rootNode = $state<LayoutNode>(_defaultLeaf);
let activePanelId = $state<string>(_defaultLeaf.panelId);

// Preset state (Story 11.4)
const PRESETS_STORAGE_KEY = 'command-center:layout-presets';
const ACTIVE_LAYOUT_STORAGE_KEY = 'command-center:active-layout';
let presets = $state(new Map<string, LayoutNode>());
let activePresetName = $state<string | null>(null);

// Derived
const isLastPanel = $derived.by(() => rootNode.type === 'leaf');

// Tree traversal helpers
export function findNode(node: LayoutNode, panelId: string): LeafNode | null {
	if (node.type === 'leaf') {
		return node.panelId === panelId ? node : null;
	}
	return findNode(node.first, panelId) || findNode(node.second, panelId);
}

export function findParent(
	node: LayoutNode,
	panelId: string,
	parent: BranchNode | null = null
): BranchNode | null {
	if (node.type === 'leaf') {
		return node.panelId === panelId ? parent : null;
	}
	return (
		findParent(node.first, panelId, node) ||
		findParent(node.second, panelId, node)
	);
}

function findFirstLeaf(node: LayoutNode): LeafNode {
	if (node.type === 'leaf') return node;
	return findFirstLeaf(node.first);
}

function findLastLeaf(node: LayoutNode): LeafNode {
	if (node.type === 'leaf') return node;
	return findLastLeaf(node.second);
}

function containsPanel(node: LayoutNode, panelId: string): boolean {
	if (node.type === 'leaf') return node.panelId === panelId;
	return (
		containsPanel(node.first, panelId) ||
		containsPanel(node.second, panelId)
	);
}

function cloneTree(node: LayoutNode): LayoutNode {
	if (node.type === 'leaf') {
		return { ...node };
	}
	return {
		...node,
		first: cloneTree(node.first),
		second: cloneTree(node.second)
	};
}

function replaceInTree(
	tree: LayoutNode,
	targetId: string,
	replacement: LayoutNode
): LayoutNode {
	if (tree.type === 'leaf') {
		return tree.panelId === targetId ? replacement : tree;
	}
	return {
		...tree,
		first: replaceInTree(tree.first, targetId, replacement),
		second: replaceInTree(tree.second, targetId, replacement)
	};
}

function removeFromTree(
	tree: LayoutNode,
	panelId: string
): LayoutNode | null {
	if (tree.type === 'leaf') {
		return tree.panelId === panelId ? null : tree;
	}

	if (
		tree.first.type === 'leaf' &&
		tree.first.panelId === panelId
	) {
		return cloneTree(tree.second);
	}
	if (
		tree.second.type === 'leaf' &&
		tree.second.panelId === panelId
	) {
		return cloneTree(tree.first);
	}

	const newFirst = removeFromTree(tree.first, panelId);
	if (newFirst !== tree.first && newFirst !== null) {
		return { ...tree, first: newFirst };
	}

	const newSecond = removeFromTree(tree.second, panelId);
	if (newSecond !== tree.second && newSecond !== null) {
		return { ...tree, second: newSecond };
	}

	return tree;
}

function updateLeafInTree(
	tree: LayoutNode,
	panelId: string,
	updater: (leaf: LeafNode) => LeafNode
): LayoutNode {
	if (tree.type === 'leaf') {
		return tree.panelId === panelId ? updater({ ...tree }) : tree;
	}
	return {
		...tree,
		first: updateLeafInTree(tree.first, panelId, updater),
		second: updateLeafInTree(tree.second, panelId, updater)
	};
}

function updateBranchRatio(
	tree: LayoutNode,
	panelId: string,
	ratio: number
): LayoutNode {
	if (tree.type === 'leaf') return tree;
	if (containsPanel(tree.first, panelId) || containsPanel(tree.second, panelId)) {
		// This is the direct parent branch
		if (
			(tree.first.type === 'leaf' && tree.first.panelId === panelId) ||
			(tree.second.type === 'leaf' && tree.second.panelId === panelId)
		) {
			return { ...tree, ratio };
		}
		// Check deeper
		if (containsPanel(tree.first, panelId)) {
			return { ...tree, first: updateBranchRatio(tree.first, panelId, ratio) };
		}
		return { ...tree, second: updateBranchRatio(tree.second, panelId, ratio) };
	}
	return tree;
}

function regeneratePanelIds(node: LayoutNode): LayoutNode {
	if (node.type === 'leaf') {
		return { ...node, panelId: generatePanelId() };
	}
	return {
		...node,
		first: regeneratePanelIds(node.first),
		second: regeneratePanelIds(node.second)
	};
}

// Exported getter functions
export function getRootNode(): LayoutNode {
	return rootNode;
}

export function getActivePanelId(): string {
	return activePanelId;
}

export function getIsLastPanel(): boolean {
	return isLastPanel;
}

export function getPresets(): Map<string, LayoutNode> {
	return presets;
}

export function getActivePresetName(): string | null {
	return activePresetName;
}

// Mutations
export function splitPanel(
	panelId: string,
	direction: 'horizontal' | 'vertical'
): void {
	const leaf = findNode(rootNode, panelId);
	if (!leaf) return;

	const newLeaf: LeafNode = {
		type: 'leaf',
		panelType: 'services',
		panelId: generatePanelId()
	};

	const branch: BranchNode = {
		type: 'branch',
		direction,
		ratio: 0.5,
		first: { ...leaf },
		second: newLeaf
	};

	if (rootNode.type === 'leaf' && rootNode.panelId === panelId) {
		rootNode = branch;
	} else {
		rootNode = replaceInTree(rootNode, panelId, branch);
	}
	saveActiveLayout();
}

export function closePanel(panelId: string): void {
	if (rootNode.type === 'leaf') return;

	const result = removeFromTree(rootNode, panelId);
	if (result) {
		rootNode = result;
		if (activePanelId === panelId) {
			activePanelId = findFirstLeaf(rootNode).panelId;
		}
	}
	saveActiveLayout();
}

export function resizePanel(panelId: string, ratio: number): void {
	const clamped = Math.min(0.9, Math.max(0.1, ratio));
	const parent = findParent(rootNode, panelId);
	if (!parent) return;

	rootNode = updateBranchRatio(rootNode, panelId, clamped);
	saveActiveLayout();
}

export function setActivePanel(panelId: string): void {
	activePanelId = panelId;
}

export function setPanelType(panelId: string, panelType: PanelType): void {
	const leaf = findNode(rootNode, panelId);
	if (!leaf) return;

	rootNode = updateLeafInTree(rootNode, panelId, (l) => ({
		...l,
		panelType
	}));
	saveActiveLayout();
}

// Spatial focus navigation (Story 11.3)
export function focusAdjacentPanel(
	direction: 'up' | 'down' | 'left' | 'right'
): void {
	const branchDir =
		direction === 'left' || direction === 'right'
			? 'horizontal'
			: 'vertical';

	// Walk up tree to find a branch aligned with the direction
	// where active panel is on the departing side
	const path: { node: BranchNode; side: 'first' | 'second' }[] = [];
	function buildPath(
		node: LayoutNode,
		target: string
	): boolean {
		if (node.type === 'leaf') return node.panelId === target;
		if (buildPath(node.first, target)) {
			path.push({ node, side: 'first' });
			return true;
		}
		if (buildPath(node.second, target)) {
			path.push({ node, side: 'second' });
			return true;
		}
		return false;
	}

	buildPath(rootNode, activePanelId);

	// Walk path (bottom to top) to find the right branch
	for (const { node, side } of path) {
		if (node.direction !== branchDir) continue;

		const isDepartingFirst =
			(direction === 'right' || direction === 'down') && side === 'first';
		const isDepartingSecond =
			(direction === 'left' || direction === 'up') && side === 'second';

		if (isDepartingFirst) {
			const target =
				direction === 'right' || direction === 'down'
					? findFirstLeaf(node.second)
					: findLastLeaf(node.second);
			activePanelId = target.panelId;
			return;
		}
		if (isDepartingSecond) {
			const target =
				direction === 'left' || direction === 'up'
					? findLastLeaf(node.first)
					: findFirstLeaf(node.first);
			activePanelId = target.panelId;
			return;
		}
	}
}

// Preset functions (Story 11.4)
export function savePreset(name: string): void {
	const trimmed = name.trim();
	if (!trimmed) return;

	const snapshot = JSON.parse(JSON.stringify(rootNode)) as LayoutNode;
	const newPresets = new Map(presets);
	newPresets.set(trimmed, snapshot);
	presets = newPresets;
	activePresetName = trimmed;
	persistPresets();
}

export function restorePreset(name: string): void {
	const preset = presets.get(name);
	if (!preset) return;

	const restored = regeneratePanelIds(
		JSON.parse(JSON.stringify(preset)) as LayoutNode
	);
	rootNode = restored;
	activePanelId = findFirstLeaf(rootNode).panelId;
	activePresetName = name;
	saveActiveLayout();
}

export function deletePreset(name: string): void {
	const newPresets = new Map(presets);
	newPresets.delete(name);
	presets = newPresets;
	if (activePresetName === name) {
		activePresetName = null;
	}
	persistPresets();
}

export function loadPresetsFromStorage(): void {
	try {
		const stored = localStorage.getItem(PRESETS_STORAGE_KEY);
		if (stored) {
			const parsed = JSON.parse(stored) as Record<string, LayoutNode>;
			const loaded = new Map<string, LayoutNode>(Object.entries(parsed));
			if (loaded.size > 0) {
				presets = loaded;
				return;
			}
		}
	} catch {
		// Fall back to default
	}
	seedDefaultPresets();
}

function seedDefaultPresets(): void {
	const monitoring: LeafNode = {
		type: 'leaf',
		panelType: 'services',
		panelId: generatePanelId()
	};
	presets = new Map([['Monitoring', monitoring]]);
	persistPresets();
}

function persistPresets(): void {
	try {
		const obj: Record<string, LayoutNode> = {};
		for (const [k, v] of presets) {
			obj[k] = v;
		}
		localStorage.setItem(PRESETS_STORAGE_KEY, JSON.stringify(obj));
	} catch {
		// localStorage may be unavailable
	}
}

export function saveActiveLayout(): void {
	try {
		localStorage.setItem(
			ACTIVE_LAYOUT_STORAGE_KEY,
			JSON.stringify(rootNode)
		);
	} catch {
		// localStorage may be unavailable
	}
}

export function loadActiveLayout(): boolean {
	try {
		const stored = localStorage.getItem(ACTIVE_LAYOUT_STORAGE_KEY);
		if (stored) {
			const parsed = JSON.parse(stored) as LayoutNode;
			const restored = regeneratePanelIds(parsed);
			rootNode = restored;
			activePanelId = findFirstLeaf(rootNode).panelId;
			return true;
		}
	} catch {
		// Fall back to default
	}
	return false;
}

// Test helper
export function _resetForTesting(): void {
	const defaultLeaf = createDefaultLeaf();
	rootNode = defaultLeaf;
	activePanelId = defaultLeaf.panelId;
	presets = new Map();
	activePresetName = null;
}
