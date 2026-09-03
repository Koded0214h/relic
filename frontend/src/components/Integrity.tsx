import './Integrity.css'

function Integrity() {

    return (
        <div className="integrity">
            <div className="integrity-content">
                <div className="left">
                    <h4>The integrity promise</h4>

                    <h6>The Dark Room rule</h6>

                    <p>
                        In a physical dark room, the negative is sacred. You do not cut it, you do not alter it. You derive prints from it.
                    </p>

                    <p>
                        Relic operates on this exact principle. When we archive a file, we generate a mathematical fingerprint. If you 
                        ever need to restore the original asset, we guarantee the output hash matches the input hash perfectly. 
                        Not a single bit is changed.
                    </p>

                    <ul>
                        <li>SHA-256 Hashing on ingest</li>
                        <li>Non-destructive Compression</li>
                        <li>Periodic integrity Scrubbing</li>
                    </ul>
                </div>

                <div className="right">

                </div>
            </div>
        </div>
    )
}

export default Integrity;