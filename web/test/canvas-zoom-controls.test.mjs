import { expect, test } from "bun:test";
import { fileURLToPath } from "node:url";
import { runInNewContext } from "node:vm";

// Execute the production component and viewport subscription. Only rendering
// and DOM primitives are substituted, as in the existing gesture tests.
const build = await Bun.build({
    entrypoints: [fileURLToPath(new URL("../src/components/canvas/canvas-zoom-controls.tsx", import.meta.url))],
    target: "browser",
    format: "cjs",
    plugins: [{
        name: "zoom-control-renderer",
        setup(builder) {
            builder.onResolve({ filter: /^react$|^react\/jsx-(?:dev-)?runtime$|^motion\/react$|^lucide-react$|floating-dock$|use-canvas-theme-store$/ }, ({ path }) => ({ path, namespace: "test" }));
            builder.onLoad({ filter: /.*/, namespace: "test" }, ({ path }) => ({
                contents: path === "react" ? "export const { useState, useRef, useEffect } = globalThis.hooks;"
                    : path.startsWith("react/") ? "export const jsx = (type, props) => ({ type, props }); export const jsxs = jsx; export const jsxDEV = jsx;"
                    : path === "motion/react" ? "export const AnimatePresence = 'presence'; export const motion = {div:'div',button:'button'};"
                    : path === "lucide-react" ? "export const Compass='svg',Eye='svg',EyeOff='svg',Focus='svg',HelpCircle='svg',LayoutTemplate='svg',Minus='svg',Plus='svg';"
                    : path.endsWith("floating-dock") ? "export const FloatingDock = 'dock';"
                    : "export const useActiveTheme = () => 'dark';",
                loader: "js",
            }));
        },
    }],
});
expect(build.success).toBe(true);
const source = await build.outputs[0].text();

function harness(scale = 0.5) {
    const cells = [];
    const effects = [];
    const dom = new Map();
    const changed = [];
    const container = new EventTarget();
    const containerRef = { current: container };
    let cursor = 0;
    let tree;
    let items;
    let range;
    let panel;
    const hooks = {
        useRef(initial) { const i = cursor++; return (cells[i] ||= { current: initial }); },
        useState(initial) {
            const i = cursor++;
            if (!(i in cells)) cells[i] = initial;
            return [cells[i], next => { cells[i] = typeof next === "function" ? next(cells[i]) : next; }];
        },
        useEffect(fn, deps) {
            const i = cursor++;
            const previous = cells[i];
            if (previous && deps.every((value, index) => Object.is(value, previous.deps[index]))) return;
            effects.push(() => { previous?.cleanup?.(); cells[i] = { deps, cleanup: fn() }; });
        },
    };
    const module = { exports: {} };
    runInNewContext(source, { module, exports: module.exports, hooks, Event, CustomEvent, Node: class {}, document: new EventTarget(), console });
    function attach(element, mounted) {
        if (!element || typeof element !== "object") return;
        if (Array.isArray(element)) { element.forEach(child => attach(child, mounted)); return; }
        const props = element.props;
        if (!props) return;
        if (props.ref) {
            const ref = props.ref;
            const text = typeof props.children === "number" || typeof props.children === "string" ? String(props.children) : Array.isArray(props.children) && props.children.every(child => typeof child === "number" || typeof child === "string") ? props.children.join("") : "";
            let node = dom.get(ref);
            if (!node) {
                node = {
                    textNode: { value: text }, renderedText: text, rawValue: String(props.defaultValue ?? ""), textWrites: 0, valueWrites: 0,
                    get textContent() { return this.textNode.value; },
                    set textContent(value) { this.textWrites++; this.textNode = { value }; },
                    get value() { return this.rawValue; },
                    set value(value) { this.valueWrites++; this.rawValue = value; },
                    contains: () => true,
                };
                dom.set(ref, node);
            } else if (node.renderedText !== text) {
                node.textContent = text;
                node.renderedText = text;
            }
            ref.current = node;
            mounted.add(ref);
            if (element.type === "input") range = node;
            if (element.type === "span" && props.className?.includes("rounded-full")) panel = node;
        }
        if (element.type === "dock") { items = props.items; props.items.forEach(item => attach(item.icon, mounted)); }
        attach(props.children, mounted);
    }
    const props = { scale, containerRef, onScaleChange: next => changed.push(next), onFitContent() {}, isMiniMapOpen: false, onToggleMiniMap() {}, onOpenShortcuts() {} };
    function render() {
        cursor = 0;
        tree = module.exports.CanvasZoomControls(props);
        const mounted = new Set();
        range = panel = undefined;
        attach(tree, mounted);
        for (const ref of dom.keys()) if (!mounted.has(ref)) { ref.current = null; dom.delete(ref); }
        while (effects.length) effects.shift()();
    }
    render();
    return {
        preview(k) { container.dispatchEvent(new CustomEvent("canvas:viewport-preview", { detail: { x: 60, y: 40, k } })); },
        click(id) { items.find(item => item.id === id).onClick(); render(); },
        commit(next) { props.scale = next; render(); },
        get dock() { return items.find(item => item.id === "zoom-precision").icon.props.children[0].props.ref.current; },
        get range() { return range; },
        get panel() { return panel; },
        changed,
    };
}

test("initial mount and precision panel display the existing percentage without repeated writes", () => {
    const h = harness(0.666);
    expect(h.dock.textContent).toBe("67");
    expect(h.dock.textWrites).toBe(0);
    h.click("zoom-precision");
    expect(h.panel.textContent).toBe("67%");
    expect(h.range.value).toBe("67");
});

test("120 pan previews leave text nodes and the range value untouched", () => {
    const h = harness();
    h.click("zoom-precision");
    const dockText = h.dock.textNode;
    const panelText = h.panel.textNode;
    for (let i = 0; i < 120; i++) h.preview(0.5);
    expect(h.dock.textNode).toBe(dockText);
    expect(h.panel.textNode).toBe(panelText);
    expect([h.dock.textWrites, h.panel.textWrites, h.range.valueWrites]).toEqual([0, 0, 0]);
});

test("real zoom updates each control and avoids writing its already correct value", () => {
    const h = harness();
    h.click("zoom-precision");
    h.range.rawValue = "80";
    h.preview(0.8);
    expect(h.dock.textContent).toBe("80");
    expect(h.panel.textContent).toBe("80%");
    expect(h.range.value).toBe("80");
    expect([h.dock.textWrites, h.panel.textWrites, h.range.valueWrites]).toEqual([1, 1, 0]);
    h.commit(0.8);
    expect(h.dock.textContent).toBe("80");
    expect(h.panel.textContent).toBe("80%");
});

test("a zoom change with the same percentage still updates the real scale used by commands", () => {
    const h = harness(0.501);
    h.preview(0.504);
    expect(h.dock.textWrites).toBe(0);
    h.click("zoom-in");
    expect(h.changed[0]).toBeCloseTo(0.604, 12);
});

test("opening and reopening precision uses the live scale even before a committed render", () => {
    const h = harness();
    h.preview(0.734);
    h.click("zoom-precision");
    expect(h.panel.textContent).toBe("73%");
    expect(h.range.value).toBe("73");
    h.click("zoom-precision");
    h.preview(0.916);
    h.click("zoom-precision");
    expect(h.panel.textContent).toBe("92%");
    expect(h.range.value).toBe("92");
});
