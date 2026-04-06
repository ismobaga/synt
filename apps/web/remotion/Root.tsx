import { Composition } from 'remotion'
import { SyntProgrammaticVideo } from './SyntProgrammaticVideo'
import { sampleVideoProps } from './sample-props'

export const RemotionRoot = () => {
    return (
        <Composition
            id="SyntProgrammaticVideo"
            component={SyntProgrammaticVideo}
            durationInFrames={sampleVideoProps.durationInFrames}
            fps={sampleVideoProps.fps}
            width={sampleVideoProps.width}
            height={sampleVideoProps.height}
            defaultProps={sampleVideoProps}
        />
    )
}
