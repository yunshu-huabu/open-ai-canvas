import assert from "node:assert/strict";
import test from "node:test";
// @ts-expect-error -- Node 原生 TypeScript 测试运行器需要保留扩展名。
import { creationResultAssetIds } from "../../lib/canvas/canvas-asset-handoff.ts";

// Node 原生 TypeScript 测试运行器要求保留扩展名，项目编译器不允许该写法。
// @ts-expect-error -- Node 原生 TypeScript 测试运行器需要保留扩展名。
import { creationAssetKey, creationResultDisplayUrls, isSameCreationAsset } from "./creation-assets.ts";
import type { Asset } from "../../stores/use-asset-store.ts";

test("同一任务的同一结果只能匹配同一个素材", () => {
    const identity = { taskId: "task-1", resultIndex: 0 };
    const asset = { metadata: { creationAssetKey: creationAssetKey(identity) } };

    assert.equal(isSameCreationAsset(asset, identity), true);
    assert.equal(isSameCreationAsset(asset, { taskId: "task-1", resultIndex: 1 }), false);
});

test("不同任务的结果不能被误判为重复素材", () => {
    const asset = { metadata: { creationAssetKey: creationAssetKey({ taskId: "task-1", resultIndex: 0 }) } };

    assert.equal(isSameCreationAsset(asset, { taskId: "task-2", resultIndex: 0 }), false);
});

test("修复前只保存任务 ID 的素材首个结果仍可被识别", () => {
    const asset = { metadata: { source: "create-generation", taskId: "task-legacy" } };

    assert.equal(isSameCreationAsset(asset, { taskId: "task-legacy", resultIndex: 0 }), true);
    assert.equal(isSameCreationAsset(asset, { taskId: "task-legacy", resultIndex: 1 }), false);
});

test("刷新后创作视频使用持久资源地址，不继续使用旧的临时结果地址", () => {
    const assets = [
        { id: "remote", kind: "video", metadata: { source: "generation-task", messageId: "message-1", taskId: "task-1", outputIndex: 0 }, data: { storageKey: "resource:video-1", url: "https://expired.example/video.mp4" } },
        { id: "local", kind: "video", metadata: { source: "generation-task", messageId: "message-1", taskId: "task-2", outputIndex: 0 }, data: { storageKey: "generation:user:video-2", url: "blob:current-session" } },
    ] as unknown as Asset[];
    const resultUrls = ["https://expired.example/video.mp4", "blob:previous-session"];
    const assetIds = creationResultAssetIds(assets, { messageId: "message-1", taskIds: ["task-1", "task-2"], resultUrls });

    assert.deepEqual(assetIds, ["remote", "local"]);
    assert.deepEqual(creationResultDisplayUrls(assets, resultUrls, assetIds), ["/api/resources/video-1/file", "blob:current-session"]);
    assert.deepEqual(creationResultDisplayUrls(assets, resultUrls, ["remote"]), resultUrls);
});
