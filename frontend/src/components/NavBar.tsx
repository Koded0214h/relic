import {useState} from 'react';
import {FaBars, FaTimes} from 'react-icons/fa';

import './NavBar.css';

function NavBar() {

    const [isOpen, setIsOpen] = useState(false)

    const toggle = () => {
        setIsOpen(!isOpen)
    }

    return (
        <div className="navbar">
            <div className="logo">
                <h1>Relic.</h1>
            </div>

            <button className="hamburger-menu" onClick={toggle}>
                {isOpen ? <FaTimes size={24} className='menu' /> : <FaBars size={24} className='menu' />}
            </button>

            <div className={`nav-right ${isOpen ? 'show': ''}`}>
                <ul className="nav-links">
                    <li><a href="#">Product</a></li>
                    <li><a href="#">How it works</a></li>
                    <li><a href="#">About</a></li>
                </ul>

                <ul className='get-started'>
                    <li><a href="#">Join the waitlist</a></li>
                </ul>
            </div>
        </div>
    )
}

export default NavBar;