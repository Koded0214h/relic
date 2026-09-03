import './Process.css'

type Features = {
    id: number,
    image: string,
    headtext: string,
    bodytext: string,
    phase: string
}

const features: Features[] = [
    {
        id: 1,
        image: "../../public/file_upload.png",
        headtext: "Ingest",
        bodytext: "Securely transfer raw assets into the staging area. The system immediately calculates cryptographic hashes to establish provenance before any processing begins.",
        phase: "Intake"
    },
    {
        id: 2,
        image: "../../public/analyze.png",
        headtext: "Analyze & Dedup",
        bodytext: "Deep inspection of metadata and pixel data to identify exact and perceptual duplicates. Redundant data is purged, preserving only the highest fidelity original.",
        phase: "Processing"
    },
    {
        id: 3,
        image: "../../public/storage.png",
        headtext: "Verify & Preserve",
        bodytext: "Final assets are encoded, compressed, and committed to the long-term store. Continuous integrity checks ensure no bit rot over the decades.",
        phase: "Storage"
    }
]

function Process() {

    return (
        <div className="features">
            <div className="features-content">

            <div className="features-head">
                <h5>The archival process</h5>
                <p>A methodical approach to digital preservation.</p>
            </div>

            <div className="cards">
                {features.map((feature) => {
                    return <div key={feature.id} className="card">
                                <div className="top">
                                    <h1>0{feature.id}</h1>
                                    <img src={feature.image} alt="" />
                                </div>

                                <div className="mid">
                                        <p className="hd">{feature.headtext}</p>

                                        <p>
                                            {feature.bodytext}
                                        </p>
                                </div>

                                <div className="bottom">
                                    <p>Phase: {feature.phase.toUpperCase()}</p>
                                </div>
                            </div>
                })}
            </div>
        </div>
        </div>
    )
}

export default Process;