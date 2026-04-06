export interface RemotionScene {
    title: string
    body: string
    accent: string
    durationInFrames: number
}

export interface SyntRemotionVideoProps {
    title: string
    subtitle: string
    fps: number
    width: number
    height: number
    durationInFrames: number
    palette: {
        background: string
        secondary: string
        accent: string
    }
    scenes: RemotionScene[]
}

const scenes: RemotionScene[] = [
    {
        title: 'Hook the viewer fast',
        body: 'Lead with one strong idea or surprising stat in the first two seconds.',
        accent: '#8b5cf6',
        durationInFrames: 90,
    },
    {
        title: 'Layer visuals and captions',
        body: 'Use bold captions, motion, and scene transitions that match the script rhythm.',
        accent: '#ec4899',
        durationInFrames: 90,
    },
    {
        title: 'End with a CTA',
        body: 'Close with one clear action for Shorts, Reels, or TikTok viewers.',
        accent: '#06b6d4',
        durationInFrames: 90,
    },
]

export const sampleVideoProps: SyntRemotionVideoProps = {
    title: 'Synt × Remotion',
    subtitle: 'Programmatic video layouts with React',
    fps: 30,
    width: 1080,
    height: 1920,
    durationInFrames: scenes.reduce((total, scene) => total + scene.durationInFrames, 45),
    palette: {
        background: '#0f172a',
        secondary: '#1e1b4b',
        accent: '#8b5cf6',
    },
    scenes,
}
