import { AbsoluteFill, Sequence, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion'
import type { SyntRemotionVideoProps } from './sample-props'

const sceneCardStyle: React.CSSProperties = {
    borderRadius: 28,
    padding: '32px 28px',
    boxShadow: '0 20px 60px rgba(15, 23, 42, 0.35)',
    background: 'rgba(15, 23, 42, 0.72)',
    border: '1px solid rgba(255,255,255,0.08)',
}

export const SyntProgrammaticVideo = ({ title, subtitle, scenes, palette }: SyntRemotionVideoProps) => {
    const frame = useCurrentFrame()
    const { fps } = useVideoConfig()

    return (
        <AbsoluteFill
            style={{
                background: `linear-gradient(135deg, ${palette.background} 0%, ${palette.secondary} 55%, ${palette.accent} 100%)`,
                color: 'white',
                fontFamily: 'Inter, Arial, sans-serif',
                padding: 48,
            }}
        >
            <AbsoluteFill
                style={{
                    opacity: interpolate(frame, [0, 18], [0, 1]),
                    transform: `translateY(${interpolate(frame, [0, 18], [24, 0])}px)`,
                }}
            >
                <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: 1.5, opacity: 0.85 }}>REACT VIDEO COMPOSITION</div>
                <div style={{ marginTop: 20, maxWidth: 760, fontSize: 72, fontWeight: 800, lineHeight: 1.05 }}>{title}</div>
                <div style={{ marginTop: 18, maxWidth: 720, fontSize: 30, lineHeight: 1.4, color: 'rgba(255,255,255,0.88)' }}>{subtitle}</div>
            </AbsoluteFill>

            {scenes.map((scene, index) => {
                const start = 45 + scenes.slice(0, index).reduce((total, item) => total + item.durationInFrames, 0)
                return (
                    <Sequence key={`${scene.title}-${index}`} from={start} durationInFrames={scene.durationInFrames}>
                        <SceneCard title={scene.title} body={scene.body} accent={scene.accent} fps={fps} />
                    </Sequence>
                )
            })}
        </AbsoluteFill>
    )
}

const SceneCard = ({ title, body, accent, fps }: { title: string; body: string; accent: string; fps: number }) => {
    const frame = useCurrentFrame()
    const entrance = spring({ frame, fps, durationInFrames: 22 })

    return (
        <AbsoluteFill style={{ justifyContent: 'center', alignItems: 'center' }}>
            <div
                style={{
                    ...sceneCardStyle,
                    width: 860,
                    transform: `scale(${interpolate(entrance, [0, 1], [0.92, 1])})`,
                    opacity: entrance,
                }}
            >
                <div
                    style={{
                        display: 'inline-flex',
                        borderRadius: 999,
                        padding: '8px 14px',
                        backgroundColor: `${accent}22`,
                        color: accent,
                        fontSize: 18,
                        fontWeight: 700,
                        letterSpacing: 0.5,
                    }}
                >
                    Remotion scene
                </div>
                <div style={{ marginTop: 18, fontSize: 54, fontWeight: 800, lineHeight: 1.08 }}>{title}</div>
                <div style={{ marginTop: 16, fontSize: 28, lineHeight: 1.45, color: 'rgba(255,255,255,0.86)' }}>{body}</div>
            </div>
        </AbsoluteFill>
    )
}
