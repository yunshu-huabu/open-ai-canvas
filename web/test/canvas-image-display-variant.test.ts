import { expect, test } from "bun:test";
import { canvasImageDisplayVariant } from "../src/lib/canvas/canvas-media-preview";
import { CanvasNodeType, type CanvasNodeData } from "../src/types/canvas";

const node: CanvasNodeData = { id: "image", type: CanvasNodeType.Image, title: "image", position: { x: 100, y: 80 }, width: 720, height: 405 };

test("ordinary canvas display chooses the thumbnail by screen size and pixel ratio", () => {
    expect(canvasImageDisplayVariant(node, 0.28, 1)).toBe("thumbnail");
    expect(canvasImageDisplayVariant(node, 0.28, 2)).toBe("thumbnail");
    expect(canvasImageDisplayVariant(node, 0.28, 3)).toBe("original");
    expect(canvasImageDisplayVariant(node, 1, 1)).toBe("original");
    expect(canvasImageDisplayVariant({ ...node, width: 300, height: 1000 }, 0.5, 1)).toBe("thumbnail");
    expect(canvasImageDisplayVariant({ ...node, width: 300, height: 1000 }, 0.5, 2)).toBe("original");
});

test("pan does not change the display variant or node geometry", () => {
    const original = JSON.stringify(node);
    for (let i = 0; i < 120; i++) expect(canvasImageDisplayVariant(node, 0.28, 1)).toBe("thumbnail");
    expect(JSON.stringify(node)).toBe(original);
});
