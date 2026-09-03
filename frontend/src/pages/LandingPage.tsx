import NavBar from "../components/NavBar";
import Hero from "../components/Hero";
import Process from "../components/Process";
import Footer from "../components/Footer";
import Features from "../components/Features";
import Cta from "../components/Cta";
import Integrity from "../components/Integrity";

function LandingPage() {

    return (
        <div className="landing-page">
            <header>
                <NavBar />
                <Hero />
            </header>

            <main>
                <Features />
                <Process />
                <Integrity />
                <Cta />
            </main>

            <Footer />
        </div>
    )
}

export default LandingPage;