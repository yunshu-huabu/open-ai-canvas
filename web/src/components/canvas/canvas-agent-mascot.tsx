import { useEffect, useId, useState } from "react";
import heroImage from "./assets/agent-mascot-hero.png";
import heroBlinkImage from "./assets/agent-mascot-hero-blink.png";
import launcherImage from "./assets/agent-mascot-launcher.png";
import launcherBlinkImage from "./assets/agent-mascot-launcher-blink.png";
import "./canvas-agent-mascot.css";

export type AgentMascotActivity = "idle" | "thinking" | "generating" | "approval";

/** Reference artwork stays intact; only the eye region swaps during a blink. */
export function CanvasAgentMascot({ activity = "idle", hero = false, paused = false }: { activity?: AgentMascotActivity; hero?: boolean; paused?: boolean }) {
    const id = useId();
    const [blinking, setBlinking] = useState(false);
    const [blinkReady, setBlinkReady] = useState(false);
    const closedEyes = hero ? heroBlinkImage : launcherBlinkImage;
    useEffect(() => {
        let disposed = false;
        const preload = new Image();
        setBlinkReady(false);
        preload.onload = () => { if (!disposed) setBlinkReady(true); };
        preload.src = closedEyes;
        return () => { disposed = true; preload.onload = null; };
    }, [closedEyes]);

    useEffect(() => {
        const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
        let timer: ReturnType<typeof setTimeout>;
        const schedule = () => {
            clearTimeout(timer);
            setBlinking(false);
            if (!blinkReady || paused || motion.matches || document.hidden) return;
            timer = setTimeout(() => {
                setBlinking(true);
                timer = setTimeout(schedule, 170);
            }, 2800 + Math.random() * 4200);
        };
        schedule();
        document.addEventListener("visibilitychange", schedule);
        motion.addEventListener("change", schedule);
        return () => {
            clearTimeout(timer);
            document.removeEventListener("visibilitychange", schedule);
            motion.removeEventListener("change", schedule);
        };
    }, [blinkReady, paused]);

    return (
        <span className="canvas-agent-mascot" data-variant={hero ? "hero" : "launcher"} data-activity={activity} data-blinking={blinking || undefined} data-paused={paused || undefined} aria-hidden="true">
            <span className="mascot-halo" />
            <span className="mascot-float">
                <img className="mascot-artwork" src={hero ? heroImage : launcherImage} alt="" width={1254} height={1254} draggable={false} decoding="async" />
                {blinkReady ? <img className="mascot-blink" src={closedEyes} alt="" width={1254} height={1254} draggable={false} decoding="async" /> : null}
                <svg className="mascot-light-trail" viewBox="0 0 100 100" fill="none" focusable="false">
                    <defs><clipPath id={`${id}-front`}><rect x="-50" y="0" width="100" height="50" /></clipPath></defs>
                    <g transform={hero ? "translate(54 56) rotate(-27) scale(1 .3)" : "translate(50 52) rotate(-27) scale(1 .3)"}>
                        <g clipPath={`url(#${id}-front)`}>
                            <g className="mascot-orbit">
                                <circle r="43" stroke="var(--mascot-pearl)" strokeOpacity=".65" strokeWidth=".8" strokeDasharray="9 261" />
                                <circle cx="43" r="1" fill="var(--mascot-pearl)" />
                                <circle cx="-43" r=".8" fill="var(--mascot-cyan)" />
                            </g>
                        </g>
                    </g>
                </svg>
                <span className="mascot-thinking"><i /><i /><i /></span>
            </span>
        </span>
    );
}
