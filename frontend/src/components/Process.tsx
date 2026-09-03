import './Process.css'

function Process() {

    return (
        <div className="features">
            <div className="features-content">

            <div className="features-head">
                <h5>The archival process</h5>
                <p>A methodical approach to digital preservation.</p>
            </div>

            <div className="cards">
                <div className="card">
                    <div className="top">
                        <h1>01</h1>
                        <img src="" alt="" />
                    </div>

                    <div className="mid">
                            <p className="hd">Ingest</p>

                            <p>
                                Securely transfer raw assets into the staging area. The system immediately 
                                calculates cryptographic hashes to establish provenance before any processing begins.
                            </p>
                    </div>

                    <div className="bottom">
                        <p>Phase: Intake</p>
                    </div>
                </div>
            </div>
        </div>
        </div>
    )
}

export default Process;