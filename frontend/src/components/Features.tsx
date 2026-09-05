import './Features.css'

type Features = {
    id: number,
    img: string,
    text: string
}

const features: Features[] = [
    {
        id: 1,
        img: "../../public/code.png",
        text: "100% Open Source"
    },
    {
        id: 2,
        img: "../../public/self_hosted.png",
        text: "Self-Hosted"
    },
    {
        id: 3,
        img: "../../public/bit.png",
        text: "Bit-exact restoration"
    }
]

function Features() {

    return (
        <div className="features">
            <div className="features-content">
                {features.map((feat) => {
                    return (
                        <div className="feat" key={feat.id}>
                            <img src={feat.img} alt="" />
                            <h4>{feat.text}</h4>
                        </div>
                    )
                } )}
            </div>
        </div>
    )
}

export default Features;