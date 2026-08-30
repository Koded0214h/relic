import './NavBar.css';

function NavBar() {

    return (
        <div className="navbar">
            <div className="logo">
                <h1>Relic.</h1>
            </div>

            <ul className="nav-links">
                <li><a href="#">How it works</a></li>
            </ul>

            <ul className='get-started'>
                <li><a href="#">Join the waitlist</a></li>
            </ul>
        </div>
    )
}

export default NavBar;