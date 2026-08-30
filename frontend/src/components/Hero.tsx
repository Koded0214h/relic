import './Hero.css';

function Hero() {

    return (
        <div className="hero">
            <h1>Your Photos deserve more space</h1>
            
            <div className="sub">
                <p>Make your photography library smaller without deleting a single photograph.</p>
                <p>Relic finds duplicates, packs your archive more efficiently, and gives every file back exactly as it was.</p>
            </div>

            <div className="btns">
                <a href="#" className='have-bg'>Join the waitlist</a>
                <a href="#">See how it works</a>
            </div>
        </div>
    )
}

export default Hero;