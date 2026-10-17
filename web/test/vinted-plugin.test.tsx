import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { VideoSettingsPanel } from "../src/components/video-settings-panel";
import { canvasThemes } from "../src/lib/canvas-theme";
import { defaultModelCapabilityConfig, modelCapabilityConfigFor } from "../src/lib/model-capabilities";
import { prepareBackendGenerationTask } from "../src/services/api/generation-task";
import { defaultConfig, type AiConfig } from "../src/stores/use-config-store";

function vintedConfig(model = "seedance2.0fast"): AiConfig {
    return { ...defaultConfig, model: `vinted::${model}`, videoModel: `vinted::${model}`, size: "16:9", videoSeconds: "15", vquality: "720p", channels: [{ id: "vinted", name: "Vinted", scope: "system", baseUrl: "/api", apiKey: "", apiFormat: "openai", interfaceType: "vinted-seedance-video", models: [model], modelCosts: [{ model, capability: "video", protocol: "vinted-seedance-video", billingMode: "per_call", unitPriceMicrocredits: 1 }] }] };
}

test("Vinted model defaults expose exactly the four documented models' specifications", () => {
    const long = modelCapabilityConfigFor(vintedConfig("seedance2.5"), "vinted::seedance2.5").video!;
    expect(long.duration).toEqual({ selection: "range", min: 16, max: 30, step: 1, default: 30 });
    expect(long.references.maxImages).toBe(30);
    expect(long.references.maxAudios).toBe(10);
    expect(long.resolutions).toEqual(["720p"]);
    expect(defaultModelCapabilityConfig("vinted-seedance-video", "seedance2.0mini").video!.duration.values).toEqual([5, 10]);
});

test("Vinted real video panel exposes camera and native face controls without Ark settings", () => {
    const config = vintedConfig();
    const html = renderToStaticMarkup(<VideoSettingsPanel config={config} onConfigChange={() => {}} theme={canvasThemes.dark} />);
    expect(html).toContain("运镜");
    expect(html).toContain("原生人脸");
    expect(html).toContain("可能加收积分");
    expect(html).not.toContain("1080P");
    const long = renderToStaticMarkup(<VideoSettingsPanel config={{ ...vintedConfig("seedance2.5"), videoSeconds: "30" }} onConfigChange={() => {}} theme={canvasThemes.dark} />);
    expect(long).toContain("运镜");
    expect(long).not.toContain("原生人脸");
});

test("Vinted UI options reach the actual task metadata as correct upstream types", async () => {
    const config = { ...vintedConfig(), videoCameraMovement: "fixed", videoNativeFace: "false" };
    const task = await prepareBackendGenerationTask({ mode: "video", prompt: "a clip", config });
    expect(task.input.metadata?.providerOptions).toMatchObject({ "vinted-seedance-video": { camera_movement: "fixed", native_face: false } });
});
