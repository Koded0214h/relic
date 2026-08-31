import NavBar from "../components/NavBar";
import Hero from "../components/Hero";
import Features from "../components/Features";
import Footer from "../components/Footer";

function LandingPage() {

    return (
        <div className="landing-page">
            <header>
                <NavBar />
                <Hero />
            </header>

            <main>
                <Features />
            </main>
            
            <Footer />
        </div>
    )
}

export default LandingPage;