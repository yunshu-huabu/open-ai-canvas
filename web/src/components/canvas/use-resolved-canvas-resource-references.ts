import { useEffect, useMemo, useState } from "react";

import type { CanvasResourceReference } from "@/lib/canvas/canvas-resource-references";
import { loadCanvasDrawingPreview } from "@/lib/canvas/canvas-drawing-storage";
import { getResourceAccess, resolveResourceAccessURL, resourceIdFromStorageKey } from "@/services/api/resources";
import { resolveImageUrl } from "@/services/image-storage";

type ResolvedPreview = {
    identity: string;
    url: string;
};

export function useResolvedCanvasResourceReferences(references: CanvasResourceReference[], options?: { projectId?: string }) {
    const projectId = options?.projectId;
    const requests = useMemo(
        () => references.flatMap((reference) => {
            const identity = previewIdentity(reference, projectId);
            return identity ? [{ reference, identity }] : [];
        }),
        [projectId, references],
    );
    const [resolvedById, setResolvedById] = useState<Record<string, ResolvedPreview>>({});
    const [refreshVersion, setRefreshVersion] = useState(0);

    useEffect(() => {
        if (!requests.length) return;
        let cancelled = false;
        let refreshTimer: ReturnType<typeof setTimeout> | undefined;
        void Promise.all(
            requests.map(async ({ reference, identity }) => ({ id: reference.id, identity, ...await resolveReferencePreview(reference, projectId) })),
        ).then((resolved) => {
            if (cancelled) return;
            setResolvedById((current) => {
                let changed = false;
                const next = { ...current };
                resolved.forEach(({ id, identity, url }) => {
                    if (!url || (current[id]?.identity === identity && current[id]?.url === url)) return;
                    next[id] = { identity, url };
                    changed = true;
                });
                return changed ? next : current;
            });
            const nextRefresh = Math.min(...resolved.map((item) => item.refreshAt ?? Number.POSITIVE_INFINITY));
            if (Number.isFinite(nextRefresh)) refreshTimer = setTimeout(() => setRefreshVersion((version) => version + 1), Math.max(1_000, nextRefresh - Date.now()));
        });
        return () => {
            cancelled = true;
            if (refreshTimer) clearTimeout(refreshTimer);
        };
    }, [projectId, requests, refreshVersion]);

    return useMemo(
        () => references.map((reference) => {
            const identity = previewIdentity(reference, projectId);
            const resolved = identity ? resolvedById[reference.id] : undefined;
            return resolved?.identity === identity && resolved.url !== reference.previewUrl ? { ...reference, previewUrl: resolved.url } : reference;
        }),
        [projectId, references, resolvedById],
    );
}

function previewIdentity(reference: CanvasResourceReference, projectId?: string) {
    // 只有当 drawingId 和 projectId 同时存在时才返回绘图身份,避免不完整的缓存键导致运行时错误
    if (reference.drawingId) {
        if (!projectId) return "";
        return `drawing:${projectId}:${reference.drawingId}:${reference.drawingRevision || 0}`;
    }
    const storageKey = reference.kind === "video" ? reference.previewStorageKey : reference.storageKey;
    if (!storageKey || !["image", "video", "character"].includes(reference.kind)) return "";
    return `${reference.kind}:${storageKey}`;
}

async function resolveReferencePreview(reference: CanvasResourceReference, projectId?: string): Promise<{ url: string; refreshAt?: number }> {
    if (reference.drawingId && projectId) {
        const url = await loadCanvasDrawingPreview(projectId, reference.drawingId)
            .then((preview) => preview ? blobToDataUrl(preview) : reference.previewUrl || "")
            .catch(() => reference.previewUrl || "");
        return { url };
    }
    const storageKey = reference.kind === "video" ? reference.previewStorageKey : reference.storageKey;
    if (resourceIdFromStorageKey(storageKey)) {
        try {
            const access = await getResourceAccess(storageKey, "display");
            return { url: resolveResourceAccessURL(access.url), refreshAt: Date.parse(access.refreshAt) };
        } catch {
            return { url: reference.previewUrl || "", refreshAt: Date.now() + 30_000 };
        }
    }
    return { url: await resolveImageUrl(storageKey, reference.previewUrl || "").catch(() => reference.previewUrl || "") };
}

function blobToDataUrl(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onerror = () => reject(reader.error || new Error("读取绘图预览失败"));
        reader.onload = () => resolve(typeof reader.result === "string" ? reader.result : "");
        reader.readAsDataURL(blob);
    });
}
