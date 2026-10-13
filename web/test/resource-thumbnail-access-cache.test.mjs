import { expect, test } from "bun:test";
import { fileURLToPath } from "node:url";
import { runInNewContext } from "node:vm";

const build = await Bun.build({
    entrypoints: [fileURLToPath(new URL("../src/services/api/resources.ts", import.meta.url))],
    target: "browser",
    format: "cjs",
    plugins: [{
        name: "resource-access-boundaries",
        setup(builder) {
            builder.onResolve({ filter: /user-scope$|localforage-storage$|api\/request$/ }, ({ path }) => ({ path, namespace: "test" }));
            builder.onLoad({ filter: /.*/, namespace: "test" }, ({ path }) => ({
                contents: path.endsWith("user-scope") ? "export const getActiveUserScope = () => 'user';"
                    : path.endsWith("localforage-storage") ? "export const localForageStorageForScope = () => globalThis.storage;"
                    : "export const http = globalThis.http; export const apiBaseURL='/api'; export class ApiError extends Error {}",
                loader: "js",
            }));
        },
    }],
});
expect(build.success).toBe(true);
const source = await build.outputs[0].text();

function harness(legacyFallback = false) {
    let requests = 0;
    const fallback = { resourceId: "image", requestedVariant: "thumbnail", actualVariant: "original", fallbackReason: "thumbnail_not_ready", url: "https://media.example/original.png", delivery: "cdn", refreshAt: new Date(Date.now() + 600_000).toISOString() };
    const thumbnail = { ...fallback, actualVariant: "thumbnail", fallbackReason: undefined, url: "https://media.example/thumbnail.png", originalWidth: 3840, originalHeight: 2160 };
    const entries = new Map(legacyFallback ? [["resource-access-v1:image:thumbnail", JSON.stringify({ value: fallback, expiresAt: Date.now() + 600_000 })]] : []);
    const module = { exports: {} };
    runInNewContext(source, {
        module, exports: module.exports, setTimeout, clearTimeout, DOMException, console,
        storage: { getItem: async key => entries.get(key), setItem: async (key, value) => entries.set(key, value), removeItem: async key => entries.delete(key) },
        http: { post: async () => { requests++; return { items: [{ access: !legacyFallback && requests === 1 ? fallback : thumbnail }] }; } },
    });
    return { get: () => module.exports.getResourceAccess("resource:image", "display", "thumbnail"), get requests() { return requests; } };
}

test("an unavailable thumbnail is retried, then the ready variant is cached", async () => {
    const h = harness();
    expect((await h.get()).actualVariant).toBe("original");
    const ready = await h.get();
    expect(ready.actualVariant).toBe("thumbnail");
    expect([ready.originalWidth, ready.originalHeight]).toEqual([3840, 2160]);
    expect(await h.get()).toBe(ready);
    expect(h.requests).toBe(2);
});

test("a persisted not-ready fallback cannot keep the canvas on original images", async () => {
    const h = harness(true);
    expect((await h.get()).actualVariant).toBe("thumbnail");
    await h.get();
    expect(h.requests).toBe(1);
});
