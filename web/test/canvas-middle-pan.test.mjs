import { expect, test } from "bun:test";
import { fileURLToPath } from "node:url";
import { runInNewContext } from "node:vm";

// Run the real viewport and live layer updates; substitute only React scheduling
// and browser APIs, following the launcher gesture tests.
const build = await Bun.build({
    entrypoints: [fileURLToPath(new URL("../src/components/canvas/canvas-viewport.tsx", import.meta.url))],
    target: "browser",
    format: "cjs",
    plugins: [{
        name: "viewport-scheduler",
        setup(builder) {
            builder.onResolve({ filter: /^react$|^react\/jsx-(?:dev-)?runtime$|use-canvas-theme-store$/ }, (args) => ({ path: args.path, namespace: "test" }));
            builder.onLoad({ filter: /.*/, namespace: "test" }, ({ path }) => ({
                contents: path === "react"
                    ? "export const { useState, useRef, useEffect, useLayoutEffect, useCallback } = globalThis.hooks; export default {};"
                    : path.startsWith("react/")
                        ? "export const jsx = (type, props) => ({ type, props }); export const jsxs = jsx; export const jsxDEV = jsx;"
                        : "export const useActiveTheme = () => 'dark';",
                loader: "js",
            }));
        },
    }],
});
expect(build.success).toBe(true);
const source = await build.outputs[0].text();

function harness(k = 1, boxSelectEnabled = true) {
    const cells = [];
    const effects = [];
    const frames = new Map();
    const previews = [];
    const commits = [];
    const graphics = [];
    let cursor = 0;
    let frameId = 0;
    let captured = null;
    let viewport = { x: 25, y: -15, k };
    let element;
    const nodes = [
        Object.freeze({ id: "a", position: Object.freeze({ x: 100, y: 80 }), width: 160, height: 90 }),
        Object.freeze({ id: "b", position: Object.freeze({ x: 420, y: 200 }), width: 240, height: 120 }),
    ];
    const world = { style: {} };
    const grid = { style: {} };
    const properties = new Map([["--canvas-committed-scale", String(k)]]);
    class Element extends EventTarget {
        closest() { return null; }
    }
    const container = Object.assign(new Element(), {
        dataset: {},
        style: {
            getPropertyValue: (name) => properties.get(name) ?? "",
            setProperty: (name, value) => properties.set(name, value),
        },
        querySelector: (selector) => selector === "[data-canvas-world-layer]" ? world : grid,
        getBoundingClientRect: () => ({ left: 0, top: 0, width: 1000, height: 800 }),
        setPointerCapture: (id) => { captured = id; },
        hasPointerCapture: (id) => captured === id,
        releasePointerCapture: () => { captured = null; },
    });
    container.addEventListener("canvas:graphics-viewport-preview", (event) => graphics.push(event.detail));
    const containerRef = { current: container };
    const window = new EventTarget();
    const body = { style: {} };
    const effect = (fn, deps) => {
        const i = cursor++;
        const previous = cells[i];
        if (previous && deps.every((value, index) => Object.is(value, previous.deps[index]))) return;
        effects.push(() => {
            previous?.cleanup?.();
            cells[i] = { deps, cleanup: fn() };
        });
    };
    const hooks = {
        useState(initial) {
            const i = cursor++;
            if (!(i in cells)) cells[i] = initial;
            return [cells[i], (next) => { cells[i] = next; }];
        },
        useRef(initial) {
            const i = cursor++;
            return (cells[i] ||= { current: initial });
        },
        useEffect: effect,
        useLayoutEffect: effect,
        useCallback(fn, deps) {
            const i = cursor++;
            if (!cells[i] || deps.some((value, index) => !Object.is(value, cells[i].deps[index]))) cells[i] = { deps, fn };
            return cells[i].fn;
        },
    };
    const module = { exports: {} };
    runInNewContext(source, {
        module, exports: module.exports, hooks, window, Element, Event, CustomEvent, document: { body },
        ResizeObserver: class { observe() {} disconnect() {} },
        requestAnimationFrame: (fn) => { frames.set(++frameId, fn); return frameId; },
        cancelAnimationFrame: (id) => frames.delete(id),
        setTimeout, clearTimeout, console,
    });
    const onViewportChange = (next) => { viewport = next; commits.push(next); };
    const onViewportPreviewChange = (next) => previews.push(next);
    const render = () => {
        cursor = 0;
        element = module.exports.CanvasViewport({ containerRef, viewport, onViewportChange, onViewportPreviewChange, boxSelectEnabled, children: nodes });
        while (effects.length) effects.shift()();
    };
    render();
    return {
        fire(type, overrides = {}) {
            const event = Object.assign(new Event(type, { cancelable: true }), {
                pointerId: 1, pointerType: "mouse", isPrimary: true,
                button: type === "pointerdown" ? 1 : -1, buttons: 4,
                clientX: 300, clientY: 250,
                ...overrides,
            });
            Object.defineProperties(event, { currentTarget: { value: container }, target: { value: container } });
            if (type === "pointerdown") element.props.onPointerDown(event);
            else window.dispatchEvent(event);
            render();
            return event;
        },
        frame() {
            const pending = [...frames.values()];
            frames.clear();
            pending.forEach((fn) => fn(100));
        },
        get captured() { return captured; },
        get panning() { return element.props["data-canvas-pan-state"] === "grabbing"; },
        get current() { return previews.at(-1) ?? viewport; },
        world, grid, nodes, graphics, commits, container,
    };
}

for (const tool of [{ name: "middle", button: 1, buttons: 4, boxSelectEnabled: true }, { name: "hand", button: 0, buttons: 1, boxSelectEnabled: false }]) {
    test.each([0.5, 1, 2])(`${tool.name} drag moves background and all nodes by 60/40 screen pixels at scale %s`, (k) => {
        const h = harness(k, tool.boxSelectEnabled);
        const original = JSON.stringify(h.nodes);
        expect(h.fire("pointerdown", { button: tool.button, buttons: tool.buttons }).defaultPrevented).toBe(true);
        expect(h.captured).toBe(1);
        h.fire("pointermove", { clientX: 360, clientY: 290, buttons: tool.buttons });
        h.frame();
        expect(h.current).toEqual({ x: 85, y: 25, k });
        expect(h.world.style.transform).toBe("translate3d(85px, 25px, 0) scale(1)");
        expect(h.grid.style.backgroundPosition).toBe("85px 25px");
        expect(h.graphics.at(-1)).toEqual(h.current);
        expect(JSON.stringify(h.nodes)).toBe(original);
        expect(h.commits).toHaveLength(0);
        h.fire("pointerup", { buttons: 0 });
        expect(h.commits).toEqual([{ x: 85, y: 25, k }]);
        expect(h.panning).toBe(false);
        expect(h.captured).toBe(null);
        h.fire("pointermove", { clientX: 600, clientY: 500, buttons: 0 });
        expect(h.current).toEqual({ x: 85, y: 25, k });
    });
}

test("each drag starts at the latest pan, including release before a pending frame", () => {
    const h = harness();
    h.fire("pointerdown");
    h.fire("pointermove", { clientX: 360, clientY: 290 });
    h.fire("pointerup", { buttons: 0 });
    h.fire("pointerdown", { clientX: 700, clientY: 600 });
    h.fire("pointermove", { clientX: 700, clientY: 600 });
    expect(h.current).toEqual({ x: 85, y: 25, k: 1 });
    h.fire("pointermove", { clientX: 720, clientY: 590 });
    h.frame();
    expect(h.current).toEqual({ x: 105, y: 15, k: 1 });
});

for (const ending of ["pointercancel", "lostpointercapture"]) {
    test(`${ending} stops panning and ignores subsequent movement`, () => {
        const h = harness();
        h.fire("pointerdown");
        h.fire("pointermove", { pointerId: 2, clientX: 900 });
        expect(h.current).toEqual({ x: 25, y: -15, k: 1 });
        h.fire("pointermove", { clientX: 360, clientY: 290 });
        h.fire(ending);
        h.fire("pointermove", { clientX: 900, clientY: 800 });
        h.frame();
        expect(h.current).toEqual({ x: 85, y: 25, k: 1 });
        expect(h.panning).toBe(false);
        expect(h.commits).toHaveLength(1);
        expect(h.captured).toBe(null);
    });
}

test("releasing middle while left is still held stops before applying that pointermove", () => {
    const h = harness();
    h.fire("pointerdown");
    h.fire("pointermove", { clientX: 360, clientY: 290, buttons: 5 });
    h.fire("pointermove", { clientX: 380, clientY: 300, button: 1, buttons: 1 });
    expect(h.current).toEqual({ x: 85, y: 25, k: 1 });
    expect(h.panning).toBe(false);
    expect(h.captured).toBe(null);
});
