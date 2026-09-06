import './Footer.css'

function Footer() {

    return (
        <footer>
            <div className="footer-content">
                <div className="left">
                    <h1>Relic.</h1>
                    <p>Built by Koded and D'anonymousCoder. &copy; 2026</p>
                </div>

                <ul className="nav-links">
                    <li><a href="#">Documentation</a></li>
                    <li><a href="#">Source</a></li>
                    <li><a href="#">privacy</a></li>
                    <li><a href="#">Security</a></li>
                </ul>
            </div>
        </footer>
    )
}

export default Footer;